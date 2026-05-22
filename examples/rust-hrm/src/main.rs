mod db;
mod handlers;

// Include the pg-flux generated Rust types.
// These structs (Employee, Department, Shift, …) are produced by:
//   pg-flux gen --lang rust --functions --out gen/
#[allow(dead_code)]
mod gen {
    include!("../gen/mod.rs");
    pub mod enums    { include!("../gen/enums.rs");    }
    pub mod tables   { include!("../gen/tables.rs");   }
    pub mod views    { include!("../gen/views.rs");    }
    pub mod types    { include!("../gen/types.rs");    }
    pub mod functions { include!("../gen/functions.rs"); }
}

use actix_web::{web, App, HttpServer, middleware};

#[tokio::main]
async fn main() -> std::io::Result<()> {
    dotenvy::dotenv().ok();

    let pool = db::connect()
        .await
        .expect("could not connect to DATABASE_URL");

    let pool = web::Data::new(pool);
    let addr = "127.0.0.1:8080";
    println!("rust-hrm listening on http://{addr}");

    HttpServer::new(move || {
        App::new()
            .app_data(pool.clone())
            // ── Departments ──────────────────────────────────────────────
            .route("/departments",       web::get().to(handlers::departments::list_departments))
            .route("/departments",       web::post().to(handlers::departments::create_department))
            .route("/departments/stats", web::get().to(handlers::departments::department_stats))
            // ── Employees ────────────────────────────────────────────────
            .route("/employees",         web::get().to(handlers::employees::list_employees))
            .route("/employees",         web::post().to(handlers::employees::create_employee))
            .route("/employees/onboard", web::post().to(handlers::employees::onboard_employee))
            .route("/employees/{id}",    web::get().to(handlers::employees::get_employee))
            // ── Shifts ───────────────────────────────────────────────────
            .route("/employees/{id}/shifts", web::get().to(handlers::shifts::list_shifts))
            .route("/employees/{id}/shifts", web::post().to(handlers::shifts::create_shift))
            // ── Leave ────────────────────────────────────────────────────
            .route("/employees/{id}/leave", web::get().to(handlers::shifts::list_leave))
    })
    .bind(addr)?
    .run()
    .await
}
