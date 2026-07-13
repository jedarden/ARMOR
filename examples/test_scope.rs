use armor::parsers::yaml::syntax_detector::SyntaxDetector;

fn main() {
    let mut detector = SyntaxDetector::new();
    let yaml = r#"
services:
  web:
    host: localhost
    port: 8080
    ssl:
      enabled: true
      cert: /path/to/cert.pem
  database:
    host: db.example.com
    port: 5432
    credentials:
      username: admin
      password: secret
deployment:
  environments:
    - name: dev
      url: dev.example.com
    - name: prod
      url: prod.example.com
"#;
    let errors = detector.detect_errors(yaml);
    
    println!("Total errors: {}", errors.len());
    for error in &errors {
        println!("Line {}: {}", error.line, error.message);
    }
}
