use actix_web::{web, HttpResponse, Responder};
use serde::{Deserialize, Serialize};
use sqlx::PgPool;
use uuid::Uuid;

#[derive(Deserialize)]
pub struct CreateShiftRequest {
    pub org_id:        i64,
    pub department_id: Option<i64>,
    pub title:         String,
    /// ISO-8601 timestamps: "2026-06-01T09:00:00Z" / "2026-06-01T17:00:00Z"
    pub starts_at:     chrono::DateTime<chrono::Utc>,
    pub ends_at:       chrono::DateTime<chrono::Utc>,
    pub notes:         Option<String>,
}

/// GET /employees/{id}/shifts
pub async fn list_shifts(
    pool: web::Data<PgPool>,
    path: web::Path<Uuid>,
) -> impl Responder {
    let employee_id = path.into_inner();

    // Timestamps are stored in a tstzrange column `during`.
    // We extract lower/upper bounds for the JSON response.
    let rows = sqlx::query!(
        r#"
        SELECT
          id,
          title,
          status,
          lower(during) AS starts_at,
          upper(during) AS ends_at,
          notes
        FROM public.shifts
        WHERE employee_id = $1
        ORDER BY lower(during) DESC
        LIMIT 50
        "#,
        employee_id
    )
    .fetch_all(pool.get_ref())
    .await;

    match rows {
        Ok(r) => HttpResponse::Ok().json(
            r.iter().map(|row| serde_json::json!({
                "id":       row.id,
                "title":    row.title,
                "status":   row.status,
                "starts_at": row.starts_at,
                "ends_at":   row.ends_at,
                "notes":     row.notes,
            })).collect::<Vec<_>>()
        ),
        Err(e) => HttpResponse::InternalServerError().body(e.to_string()),
    }
}

/// POST /employees/{id}/shifts
/// If the new shift overlaps an existing one for this employee, the EXCLUDE
/// constraint `shifts_no_overlap` fires and sqlx returns an error with
/// SQLSTATE 23P01 (exclusion_violation).
pub async fn create_shift(
    pool: web::Data<PgPool>,
    path: web::Path<Uuid>,
    body: web::Json<CreateShiftRequest>,
) -> impl Responder {
    let employee_id = path.into_inner();

    let result = sqlx::query!(
        r#"
        INSERT INTO public.shifts (
            org_id, employee_id, department_id, title, during, notes
        )
        VALUES (
            $1, $2, $3, $4,
            tstzrange($5, $6, '[)'),
            $7
        )
        RETURNING id
        "#,
        body.org_id,
        employee_id,
        body.department_id,
        body.title,
        body.starts_at as chrono::DateTime<chrono::Utc>,
        body.ends_at   as chrono::DateTime<chrono::Utc>,
        body.notes,
    )
    .fetch_one(pool.get_ref())
    .await;

    match result {
        Ok(r)  => HttpResponse::Created().json(serde_json::json!({ "id": r.id })),
        Err(e) => {
            let msg = e.to_string();
            // SQLSTATE 23P01 = exclusion_violation (overlapping shift)
            if msg.contains("23P01") || msg.contains("shifts_no_overlap") {
                HttpResponse::Conflict()
                    .body("Shift overlaps with an existing shift for this employee")
            } else {
                HttpResponse::UnprocessableEntity().body(msg)
            }
        }
    }
}

/// GET /employees/{id}/leave — list leave requests.
/// Demonstrates querying a daterange column (lower/upper bounds).
#[derive(Serialize)]
pub struct LeaveRow {
    pub id:         i64,
    pub leave_type: String,
    pub starts_on:  Option<chrono::NaiveDate>,
    pub ends_on:    Option<chrono::NaiveDate>,
    pub status:     String,
}

pub async fn list_leave(
    pool: web::Data<PgPool>,
    path: web::Path<Uuid>,
) -> impl Responder {
    let employee_id = path.into_inner();

    let rows = sqlx::query!(
        r#"
        SELECT
          id,
          leave_type,
          lower(during) AS starts_on,
          upper(during) AS ends_on,
          status
        FROM public.leave_requests
        WHERE employee_id = $1
        ORDER BY lower(during) DESC
        LIMIT 50
        "#,
        employee_id
    )
    .fetch_all(pool.get_ref())
    .await;

    match rows {
        Ok(r) => HttpResponse::Ok().json(
            r.iter().map(|row| serde_json::json!({
                "id":         row.id,
                "leave_type": row.leave_type,
                "starts_on":  row.starts_on,
                "ends_on":    row.ends_on,
                "status":     row.status,
            })).collect::<Vec<_>>()
        ),
        Err(e) => HttpResponse::InternalServerError().body(e.to_string()),
    }
}
