use sqlx::PgPool;

/// Build a connection pool from DATABASE_URL.
/// The pool sets `app.org_id` and `app.user_id` GUC via SET LOCAL at the
/// start of each transaction so RLS policies and the audit trigger fire with
/// correct context.
pub async fn connect() -> Result<PgPool, sqlx::Error> {
    let url = std::env::var("DATABASE_URL")
        .expect("DATABASE_URL must be set");
    PgPool::connect(&url).await
}
