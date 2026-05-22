use actix_web::{web, HttpResponse, Responder};
use serde::{Deserialize, Serialize};
use sqlx::PgPool;
use uuid::Uuid;

/// Request body for creating an employee.
#[derive(Deserialize)]
pub struct CreateEmployeeRequest {
    pub org_id:        i64,
    pub department_id: Option<i64>,
    pub email:         String,
    pub first_name:    String,
    pub last_name:     String,
    pub hire_date:     chrono::NaiveDate,
    pub skills:        Option<Vec<String>>,
}

/// Projection returned from the employee_directory view.
/// Mirrors gen/views.rs EmployeeDirectory but with only the columns we expose.
#[derive(Serialize, sqlx::FromRow)]
pub struct EmployeeRow {
    pub id:              Uuid,
    pub first_name:      String,
    pub last_name:       String,
    pub full_name:       Option<String>,
    pub email:           Option<String>,
    pub department_name: Option<String>,
}

/// GET /employees?org_id=1&search=alice
/// Supports optional full-text or trigram fuzzy search on full_name via
/// the search_vector (GIN) and the gin_trgm_ops index respectively.
pub async fn list_employees(
    pool: web::Data<PgPool>,
    query: web::Query<std::collections::HashMap<String, String>>,
) -> impl Responder {
    let org_id: i64 = match query.get("org_id").and_then(|v| v.parse().ok()) {
        Some(id) => id,
        None => return HttpResponse::BadRequest().body("org_id required"),
    };

    let rows = if let Some(search) = query.get("search") {
        // Full-text search: uses the search_vector GIN index.
        // For fuzzy/trigram, switch to: full_name % $2 (similarity) or ILIKE.
        sqlx::query_as!(
            EmployeeRow,
            r#"
            SELECT id, first_name, last_name, full_name, email, department_name
            FROM public.employee_directory
            WHERE org_id = $1
              AND search_vector @@ plainto_tsquery('english', $2)
            ORDER BY ts_rank(search_vector, plainto_tsquery('english', $2)) DESC
            LIMIT 100
            "#,
            org_id,
            search
        )
        .fetch_all(pool.get_ref())
        .await
    } else {
        sqlx::query_as!(
            EmployeeRow,
            r#"
            SELECT id, first_name, last_name, full_name, email, department_name
            FROM public.employee_directory
            WHERE org_id = $1
            ORDER BY last_name, first_name
            LIMIT 100
            "#,
            org_id
        )
        .fetch_all(pool.get_ref())
        .await
    };

    match rows {
        Ok(r) => HttpResponse::Ok().json(r),
        Err(e) => HttpResponse::InternalServerError().body(e.to_string()),
    }
}

/// GET /employees/{id}
pub async fn get_employee(
    pool: web::Data<PgPool>,
    path: web::Path<Uuid>,
) -> impl Responder {
    let id = path.into_inner();
    let row = sqlx::query_as!(
        EmployeeRow,
        r#"
        SELECT id, first_name, last_name, full_name, email, department_name
        FROM public.employee_directory
        WHERE id = $1
        "#,
        id
    )
    .fetch_optional(pool.get_ref())
    .await;

    match row {
        Ok(Some(r)) => HttpResponse::Ok().json(r),
        Ok(None)    => HttpResponse::NotFound().finish(),
        Err(e)      => HttpResponse::InternalServerError().body(e.to_string()),
    }
}

/// POST /employees — inserts via raw INSERT (not the stored procedure).
/// For the full onboard flow (employee + first position in one transaction),
/// use POST /employees/onboard which calls the onboard_employee procedure.
pub async fn create_employee(
    pool: web::Data<PgPool>,
    body: web::Json<CreateEmployeeRequest>,
) -> impl Responder {
    let skills: Vec<String> = body.skills.clone().unwrap_or_default();
    let row = sqlx::query!(
        r#"
        INSERT INTO public.employees (
            org_id, department_id, email, first_name, last_name,
            hire_date, skills
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING id
        "#,
        body.org_id,
        body.department_id,
        body.email,
        body.first_name,
        body.last_name,
        body.hire_date,
        &skills,
    )
    .fetch_one(pool.get_ref())
    .await;

    match row {
        Ok(r) => HttpResponse::Created().json(serde_json::json!({ "id": r.id })),
        Err(e) => {
            // A domain constraint violation (bad email format) surfaces here.
            // EXCLUDE violations from positions appear in the positions handler.
            HttpResponse::UnprocessableEntity().body(e.to_string())
        }
    }
}

/// POST /employees/onboard — calls the CALL onboard_employee(...) procedure.
/// The procedure inserts the employee and first position atomically,
/// demonstrating how pg-flux's generated types document the procedure's shape.
#[derive(Deserialize)]
pub struct OnboardRequest {
    pub org_id:        i64,
    pub email:         String,
    pub first_name:    String,
    pub last_name:     String,
    pub hire_date:     chrono::NaiveDate,
    pub department_id: i64,
    pub title:         String,
    pub level:         String,
}

pub async fn onboard_employee(
    pool: web::Data<PgPool>,
    body: web::Json<OnboardRequest>,
) -> impl Responder {
    // CALL is not directly supported by sqlx query! macro; use query() instead.
    let result = sqlx::query(
        "CALL public.onboard_employee($1, $2, $3, $4, $5, $6, $7, $8::position_level)"
    )
    .bind(body.org_id)
    .bind(&body.email)
    .bind(&body.first_name)
    .bind(&body.last_name)
    .bind(body.hire_date)
    .bind(body.department_id)
    .bind(&body.title)
    .bind(&body.level)
    .execute(pool.get_ref())
    .await;

    match result {
        Ok(_)  => HttpResponse::Created().finish(),
        Err(e) => HttpResponse::UnprocessableEntity().body(e.to_string()),
    }
}
