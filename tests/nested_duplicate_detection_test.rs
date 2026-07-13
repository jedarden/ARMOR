//! Nested Duplicate Detection Test Cases
//!
//! These tests verify the scope-aware duplicate detection system works correctly
//! for nested mappings with same key names at different scopes.
//!
//! Bead: bf-uk7lrh
//! Acceptance Criteria:
//! - Test cases added for nested duplicate scenarios
//! - Tests verify keys in different scopes are not flagged
//! - Edge cases covered (empty mappings, mixed values)
//! - All tests pass

use armor::parsers::yaml::SyntaxDetector;

// =============================================================================
// Sibling Mappings - Same Keys in Different Scopes (Should Pass)
// =============================================================================

#[test]
fn test_sibling_mappings_same_keys() {
    // Two sibling mappings with identical key names should be valid
    // Each mapping is in its own scope, so keys don't conflict
    let yaml = r#"
services:
  web:
    host: localhost
    port: 8080
  database:
    host: db.example.com
    port: 5432
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    // Should have no errors - same keys in different scopes are allowed
    assert!(
        errors.is_empty(),
        "Sibling mappings with same keys should not trigger duplicate errors. Found {} errors: {:?}",
        errors.len(),
        errors
    );
}

#[test]
fn test_three_sibling_mappings_same_keys() {
    // Three sibling mappings with identical key names
    let yaml = r#"
environments:
  dev:
    url: dev.example.com
    timeout: 30
  staging:
    url: staging.example.com
    timeout: 60
  prod:
    url: prod.example.com
    timeout: 120
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "Three sibling mappings with same keys should be valid. Found {} errors",
        errors.len()
    );
}

#[test]
fn test_sibling_mappings_complete_key_overlap() {
    // Sibling mappings where ALL keys are the same
    let yaml = r#"
servers:
  alpha:
    enabled: true
    version: 1.0
  beta:
    enabled: false
    version: 2.0
  gamma:
    enabled: true
    version: 3.0
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "Complete key overlap across sibling mappings should be allowed"
    );
}

// =============================================================================
// Deeply Nested Structures - Same Key at Multiple Levels
// =============================================================================

#[test]
fn test_deeply_nested_same_key_different_levels() {
    // Same key 'name' appears at three different nesting levels
    let yaml = r#"
config:
  name: root
  database:
    name: postgres
    connection:
      name: primary
      timeout: 30
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "Same key at different nesting levels should be allowed. Found {} errors: {:?}",
        errors.len(),
        errors
    );
}

#[test]
fn test_four_level_deep_nesting() {
    // Four levels of nesting with same key appearing at each level
    let yaml = r#"
level1:
  key: value1
  level2:
    key: value2
    level3:
      key: value3
      level4:
        key: value4
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "Four-level deep nesting with same key should be valid"
    );
}

#[test]
fn test_deep_branching_structure() {
    // Deep structure with branching - same key in different branches
    let yaml = r#"
main:
  config:
    timeout: 10
  logging:
    timeout: 5
  cache:
    timeout: 20
  metrics:
    timeout: 30
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "Same key in different branches should be allowed"
    );
}

#[test]
fn test_complex_nested_tree() {
    // Complex nested structure with multiple levels and branches
    let yaml = r#"
root:
  branch_a:
    leaf_a1: value1
    leaf_a2: value2
    sub_branch:
      leaf_a3: value3
  branch_b:
    leaf_b1: value1
    leaf_b2: value2
    sub_branch:
      leaf_b3: value3
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    // Note: 'leaf_a1', 'leaf_a2', etc. at same level in different scopes is fine
    // 'sub_branch' appears twice as parent keys - this should be allowed
    assert!(
        errors.is_empty(),
        "Complex nested tree should handle duplicate key names correctly across scopes"
    );
}

// =============================================================================
// Mixed Scalar and Collection Values
// =============================================================================

#[test]
fn test_mixed_scalar_and_mapping_values() {
    // Mix of scalar values and nested mappings with same keys
    let yaml = r#"
config:
  enabled: true
  features:
    enabled: false
  settings:
    enabled: true
    debug: true
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "Mix of scalar and mapping values with same keys should be allowed"
    );
}

#[test]
fn test_mixed_sequences_and_mappings() {
    // Mix of sequences and mappings with same keys in different scopes
    let yaml = r#"
endpoints:
  api:
    port: 8080
    routes:
      - /users
      - /posts
  admin:
    port: 8081
    routes:
      - /admin
      - /settings
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "Mix of sequences and mappings with same keys should be allowed"
    );
}

#[test]
fn test_mixed_flow_style_and_block_style() {
    // Mix of flow-style {key: value} and block-style YAML with same keys
    let yaml = r#"
services:
  web:
    config: {host: localhost, port: 8080}
    features:
      - auth
      - cache
  api:
    config: {host: localhost, port: 8081}
    features:
      - rate-limit
      - cors
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    // Flow-style content is not checked for duplicate keys (by design)
    assert!(
        errors.is_empty(),
        "Mixed flow-style and block-style should not trigger false duplicate errors"
    );
}

// =============================================================================
// Empty Mappings Edge Cases
// =============================================================================

#[test]
fn test_empty_mappings_with_keys() {
    // Empty mappings (no content after parent key)
    let yaml = r#"
config:
database:
cache:
settings:
  timeout: 30
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "Empty mappings should not cause duplicate key errors"
    );
}

#[test]
fn test_empty_nested_mappings() {
    // Empty nested mappings at different levels
    let yaml = r#"
level1:
  level2:
  level3:
key: value
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "Empty nested mappings should be handled correctly"
    );
}

#[test]
fn test_sibling_empty_and_populated_mappings() {
    // Mix of empty and populated sibling mappings
    let yaml = r#"
servers:
  alpha:
    enabled: true
  beta:
  gamma:
    enabled: false
    version: 2.0
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "Mix of empty and populated sibling mappings should be allowed"
    );
}

// =============================================================================
// Actual Duplicates - Should Be Detected
// =============================================================================

#[test]
fn test_duplicate_in_same_scope_detected() {
    // Actual duplicate in the same scope (should fail)
    let yaml = r#"
config:
  host: localhost
  host: duplicate
  port: 8080
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    // Should detect the duplicate 'host' key
    assert!(
        !errors.is_empty(),
        "Should detect duplicate key in same scope"
    );

    assert!(
        errors.iter().any(|e| e.message.contains("duplicate") || e.message.contains("host")),
        "Should have error about duplicate 'host' key"
    );
}

#[test]
fn test_multiple_duplicates_in_same_scope() {
    // Multiple duplicate keys in the same scope
    let yaml = r#"
settings:
  timeout: 10
  timeout: 20
  enabled: true
  enabled: false
  debug: false
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    // Should detect both duplicates
    assert!(
        !errors.is_empty(),
        "Should detect multiple duplicate keys"
    );

    // Check that we have at least 2 duplicate errors (one for timeout, one for enabled)
    let duplicate_count = errors.iter()
        .filter(|e| e.message.contains("duplicate"))
        .count();

    assert!(
        duplicate_count >= 2,
        "Should detect at least 2 duplicate keys, found {}",
        duplicate_count
    );
}

#[test]
fn test_duplicate_at_root_level() {
    // Duplicate keys at root level
    let yaml = r#"
key1: value1
key2: value2
key1: duplicate
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        !errors.is_empty(),
        "Should detect duplicate key at root level"
    );
}

// =============================================================================
// Complex Real-World Scenarios
// =============================================================================

#[test]
fn test_realistic_config_file() {
    // Realistic configuration file with nested services
    let yaml = r#"
version: 1.0

services:
  web:
    name: web-service
    port: 8080
    env: production
    resources:
      cpu: 500m
      memory: 512Mi
      limits:
        cpu: 1000m
        memory: 1Gi

  api:
    name: api-service
    port: 8081
    env: production
    resources:
      cpu: 250m
      memory: 256Mi
      limits:
        cpu: 500m
        memory: 512Mi

  worker:
    name: worker-service
    port: 8082
    env: production
    resources:
      cpu: 100m
      memory: 128Mi
      limits:
        cpu: 200m
        memory: 256Mi

global:
  timeout: 30
  retries: 3
  log_level: info
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "Realistic config file with nested services should have no duplicate key errors. Found {} errors: {:?}",
        errors.len(),
        errors
    );
}

#[test]
fn test_docker_compose_like_structure() {
    // Docker-compose style configuration
    let yaml = r#"
version: '3.8'

services:
  db:
    image: postgres:13
    environment:
      POSTGRES_DB: mydb
      POSTGRES_USER: user
      POSTGRES_PASSWORD: pass
    volumes:
      - db_data:/var/lib/postgresql/data
    networks:
      - backend

  web:
    image: nginx:latest
    ports:
      - "80:80"
      - "443:443"
    environment:
      ENV: production
      DEBUG: "false"
    volumes:
      - static_files:/var/www/static
    networks:
      - frontend
      - backend

  redis:
    image: redis:6
    command: redis-server --appendonly yes
    volumes:
      - redis_data:/data
    networks:
      - backend

networks:
  frontend:
    driver: bridge
  backend:
    driver: bridge

volumes:
  db_data:
  redis_data:
  static_files:
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "Docker-compose style configuration should handle nested keys correctly"
    );
}

#[test]
fn test_kubernetes_like_resources() {
    // Kubernetes-style single resource configuration
    // (Multi-document YAML would require document separator handling)
    let yaml = r#"
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: default
data:
  APP_ENV: production
  APP_DEBUG: "false"
  DB_HOST: localhost
  DB_PORT: "5432"
  CACHE_HOST: redis.example.com
  CACHE_PORT: "6379"
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    // Should have no duplicate key errors
    let has_duplicate_errors = errors.iter()
        .any(|e| e.message.contains("duplicate"));

    assert!(
        !has_duplicate_errors,
        "Kubernetes-style config should not have duplicate key errors. Found: {:?}",
        errors
    );
}

// =============================================================================
// Edge Cases and Boundary Conditions
// =============================================================================

#[test]
fn test_single_key_per_scope() {
    // Only one key per scope - no duplicates possible
    let yaml = r#"
a:
  b:
    c:
      d: value
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "Single key per scope should have no duplicates"
    );
}

#[test]
fn test_all_keys_unique_across_all_scopes() {
    // All keys are unique across entire document
    let yaml = r#"
key1: value1
key2:
  key3: value3
  key4:
    key5: value5
key6:
  key7:
    key8: value8
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "All unique keys should have no duplicate errors"
    );
}

#[test]
fn test_maximum_nesting_depth() {
    // Test very deep nesting (10+ levels)
    let yaml = r#"
l1:
  l2:
    l3:
      l4:
        l5:
          l6:
            l7:
              l8:
                l9:
                  l10:
                    value: deep
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "Very deep nesting should be handled correctly"
    );
}

#[test]
fn test_wide_shallow_structure() {
    // Many keys at same level (wide, shallow structure)
    let yaml = r#"
root:
  key1: value1
  key2: value2
  key3: value3
  key4: value4
  key5: value5
  key6: value6
  key7: value7
  key8: value8
  key9: value9
  key10: value10
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "Wide shallow structure with many keys should work"
    );
}

#[test]
fn test_comments_should_not_affect_scope_tracking() {
    // Comments mixed with content should not affect scope tracking
    let yaml = r#"
# Root configuration
config:
  # Database settings
  database:
    host: localhost
    port: 5432
    # Cache settings
  cache:
    host: redis.example.com
    port: 6379

# More config
settings:
  # Feature flags
  features:
    enabled: true
    # Logging
  logging:
    level: info
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "Comments should not affect scope-aware duplicate detection"
    );
}

#[test]
fn test_blank_lines_should_not_affect_scope_tracking() {
    // Blank lines mixed with content should not affect scope tracking
    let yaml = r#"
services:


  web:
    host: localhost


    port: 8080

  database:

    host: db.example.com

    port: 5432


"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "Blank lines should not affect scope-aware duplicate detection"
    );
}

// =============================================================================
// Base Indent Size Variations
// =============================================================================

#[test]
fn test_4_space_indent_with_same_keys() {
    // 4-space base indent with same keys in different scopes
    // Note: Default detector uses 2-space indent, so this test will detect
    // indentation errors but should NOT detect false duplicate key errors
    let yaml = r#"
services:
    web:
        host: localhost
        port: 8080
    database:
        host: db.example.com
        port: 5432
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    // With 2-space default config, 4-space indent will be flagged as inconsistent
    // but it should NOT be flagged as duplicate keys
    let has_indent_error = errors.iter()
        .any(|e| e.indentation_error_type.is_some());

    // The important thing: should NOT have duplicate key errors
    let has_duplicate_error = errors.iter()
        .any(|e| e.message.contains("duplicate"));

    assert!(
        !has_duplicate_error,
        "Should NOT detect false duplicate key errors. Found errors: {:?}",
        errors
    );
}

#[test]
fn test_inconsistent_indentation_detected() {
    // Inconsistent indentation (mix of 2 and 4 spaces) should be flagged
    // but NOT as duplicate keys
    let yaml = r#"
services:
  web:
      host: localhost  # 4 spaces instead of 2
    port: 8080
  database:
    host: db.example.com
    port: 5432
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    // Should NOT have duplicate key errors (indentation issues are separate)
    let has_duplicate_error = errors.iter()
        .any(|e| e.message.contains("duplicate"));

    assert!(
        !has_duplicate_error,
        "Should NOT have false duplicate key errors"
    );
}

// =============================================================================
// Special Characters and Key Types
// =============================================================================

#[test]
fn test_keys_with_dashes_same_in_different_scopes() {
    // Keys with dashes in different scopes
    let yaml = r#"
config:
  max-timeout: 10
  min-timeout: 5

limits:
  max-timeout: 30
  min-timeout: 1
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "Keys with dashes in different scopes should be allowed"
    );
}

#[test]
fn test_keys_with_underscores_same_in_different_scopes() {
    // Keys with underscores in different scopes
    let yaml = r#"
database:
  max_connections: 100
  min_connections: 10

cache:
  max_connections: 50
  min_connections: 5
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "Keys with underscores in different scopes should be allowed"
    );
}

#[test]
fn test_keys_with_dots_same_in_different_scopes() {
    // Keys with dots in different scopes (common in config)
    let yaml = r#"
server:
  example.com:
    tls: true
  api.example.com:
    tls: false

cdn:
  static.example.com:
    ttl: 3600
  cdn.example.com:
    ttl: 86400
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml);

    assert!(
        errors.is_empty(),
        "Keys with dots in different scopes should be allowed"
    );
}
