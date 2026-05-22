use actix_web::{web, HttpResponse, Responder};
use serde::{Deserialize, Serialize};
use sqlx::PgPool;

/// Projection returned for a department listing.
/// Includes parent_id to let clients reconstruct the hierarchy.
#[derive(Serialize, sqlx::FromRow)]
pub struct DepartmentRow {
    pub id:          i64,
    pub org_id:      i64,
    pub parent_id:   Option<i64>,
    pub name:        String,
    pub code:        String,
    pub depth:       i16,
}

/// GET /departments?org_id=1
pub async fn list_departments(
    pool: web::Data<PgPool>,
    query: web::Query<std::collections::HashMap<String, String>>,
) -> impl Responder {
    let org_id: i64 = match query.get("org_id").and_then(|v| v.parse().ok()) {
        Some(id) => id,
        None => return HttpResponse::BadRequest().body("org_id required"),
    };

    // ORDER BY depth, name gives a natural tree-ordered listing.
    let rows = sqlx::query_as!(
        DepartmentRow,
        r#"
        SELECT id, org_id, parent_id, name, code, depth
        FROM public.departments
        WHERE org_id = $1
        ORDER BY depth, name
        "#,
        org_id
    )
    .fetch_all(pool.get_ref())
    .await;

    match rows {
        Ok(r) => HttpResponse::Ok().json(r),
        Err(e) => HttpResponse::InternalServerError().body(e.to_string()),
    }
}

#[derive(Deserialize)]
pub struct CreateDepartmentRequest {
    pub org_id:    i64,
    pub parent_id: Option<i64>,
    pub name:      String,
    pub code:      String,
}

/// POST /departments — creates a department.
/// depth is auto-computed from parent; a NULL parent_id means root (depth 0).
pub async fn create_department(
    pool: web::Data<PgPool>,
    body: web::Json<CreateDepartmentRequest>,
) -> impl Responder {
    let row = sqlx::query!(
        r#"
        WITH parent AS (
          SELECT coalesce(max(depth), -1) AS parent_depth
          FROM public.departments
          WHERE id = $2
        )
        INSERT INTO public.departments (org_id, parent_id, name, code, depth)
        SELECT $1, $2, $3, $4, parent_depth + 1 FROM parent
        RETURNING id
        "#,
        body.org_id,
        body.parent_id,
        body.name,
        body.code,
    )
    .fetch_one(pool.get_ref())
    .await;

    match row {
        Ok(r)  => HttpResponse::Created().json(serde_json::json!({ "id": r.id })),
        Err(e) => HttpResponse::UnprocessableEntity().body(e.to_string()),
    }
}

/// GET /departments/stats — returns the materialized department_stats view.
/// Includes size_rank (window function) showing each dept's rank by head-count
/// within its organisation.
#[derive(Serialize, sqlx::FromRow)]
pub struct DeptStatsRow {
    pub department_id:   i64,
    pub department_name: String,
    pub org_id:          i64,
    pub employee_count:  Option<i64>,
    pub active_count:    Option<i64>,
    pub size_rank:       Option<i64>,
}

pub async fn department_stats(
    pool: web::Data<PgPool>,
    query: web::Query<std::collections::HashMap<String, String>>,
) -> impl Responder {
    let org_id: i64 = match query.get("org_id").and_then(|v| v.parse().ok()) {
        Some(id) => id,
        None => return HttpResponse::BadRequest().body("org_id required"),
    };

    let rows = sqlx::query_as!(
        DeptStatsRow,
        r#"
        SELECT department_id, department_name, org_id,
               employee_count, active_count,
               size_rank::bigint AS "size_rank: _"
        FROM public.department_stats
        WHERE org_id = $1
        ORDER BY size_rank
        "#,
        org_id
    )
    .fetch_all(pool.get_ref())
    .await;

    match rows {
        Ok(r) => HttpResponse::Ok().json(r),
        Err(e) => HttpResponse::InternalServerError().body(e.to_string()),
    }
}
