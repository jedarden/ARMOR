//! YAML Comment False Positive Prevention Tests
//!
//! These tests verify that hash symbols (#) in legitimate contexts are NOT treated as comments:
//! - Hash symbols in scalar values (hex codes, config values)
//! - URLs with anchors (containing #)
//! - YAML anchors and aliases (using & and *)
//! - Tags containing hash-like patterns
//!
//! Bead: bf-sxg2u
//! Acceptance Criteria:
//! - Test for hash in scalar values not treated as comment
//! - Test for URLs with anchors not split at #
//! - Test for YAML anchors/aliases handled correctly
//! - Test for tags with hash-like patterns
//! - All tests pass

use armor::parsers::yaml::{
    classify_line_type, is_comment_line, strip_inline_comment, detect_mapping_key, LineType
};

// Hash Symbols in Scalar Values Tests

#[test]
fn test_hash_in_hex_color_code() {
    // Hex color codes MUST be quoted to preserve the hash
    // Unquoted hash preceded by whitespace IS a comment per YAML spec
    let test_cases = vec![
        "color: \"#FFFFFF\"",
        "background: '#abc123'",
        "border: \"#FFF\"",
    ];

    for line in test_cases {
        // Should NOT be a comment line
        assert!(!is_comment_line(line));

        // Should be classified as mapping key, not comment
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        // Hash should be preserved when value is quoted
        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('#'));

        // Key detection should work correctly
        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert!(info.value.unwrap().contains('#'));
    }
}

#[test]
fn test_unquoted_hash_is_comment() {
    // Unquoted hash preceded by whitespace IS a comment (YAML spec)
    let test_cases = vec![
        ("color: #FFFFFF", "color: "),
        ("background: #abc123", "background: "),
        ("border: #FFF", "border: "),
    ];

    for (line, expected_stripped) in test_cases {
        // Should NOT be a comment line (has content before #)
        assert!(!is_comment_line(line));

        // Hash should be stripped as it's a comment
        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected_stripped);

        // Key detection should work but value should be empty or just whitespace
        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
        let info = info.unwrap();
        // Value is either None or empty/whitespace since # starts comment
        assert!(info.value.is_none() || info.value.unwrap().trim().is_empty());
    }
}

#[test]
fn test_hash_in_config_values() {
    // Hash symbols in configuration values MUST be quoted
    let test_cases = vec![
        "separator: \"#\"",
        "comment_char: '#'",
        "marker: \"###\"",
        "prefix: \"#!\"",
        "symbol: \"$#\"",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line), "Should not be comment line: {}", line);
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('#'), "Hash should be preserved in: {}", line);

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some(), "Should detect key in: {}", line);
    }
}

#[test]
fn test_hash_at_end_of_value() {
    // Hash at the end of a value (without preceding whitespace)
    let test_cases = vec![
        "heading: level1#",
        "status: active#",
        "flag: true#",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line));
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        assert!(stripped.ends_with('#'), "Hash at end should be preserved in: {}", line);

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert!(info.value.unwrap().ends_with('#'));
    }
}

#[test]
fn test_hash_in_middle_of_value() {
    // Hash in the middle of a value (without preceding whitespace)
    let test_cases = vec![
        "pattern: value#TODO",
        "template: value###value###",
        "markup: text#more#text",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line));

        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('#'), "Hash in middle should be preserved in: {}", line);

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
    }
}

#[test]
fn test_hash_with_space_followed_by_value() {
    // Hash followed by space then more value (hash is part of value, not comment)
    // When # is NOT preceded by whitespace, it's part of the value
    let line = "key: value# continued";

    assert!(!is_comment_line(line));

    let stripped = strip_inline_comment(line);
    assert_eq!(stripped, "key: value# continued");

    let info = detect_mapping_key(line, 0);
    assert!(info.is_some());
    let info = info.unwrap();
    assert_eq!(info.value, Some("value# continued".to_string()));
}

// URL with Anchor Tests

#[test]
fn test_url_with_anchor_hash() {
    // URLs with anchor/fragment identifiers
    let test_cases = vec![
        "url: http://example.com#section",
        "link: https://example.com/path#anchor",
        "href: https://api.example.com/v1#endpoint",
        "docs: http://localhost:8080/docs#readme",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line), "URL with anchor should not be comment: {}", line);
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('#'), "Anchor hash should be preserved in: {}", line);

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert!(info.value.unwrap().contains('#'));
    }
}

#[test]
fn test_url_with_anchor_and_inline_comment() {
    // URLs with anchors AND actual inline comments
    let test_cases = vec![
        ("url: http://example.com#section # documentation", "url: http://example.com#section "),
        ("link: https://example.com#anchor # this is comment", "link: https://example.com#anchor "),
    ];

    for (line, expected_stripped) in test_cases {
        assert!(!is_comment_line(line));

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected_stripped);

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
        let info = info.unwrap();
        // Value should preserve URL anchor but strip the comment
        assert!(info.value.unwrap().contains('#'));
    }
}

#[test]
fn test_multiple_anchors_in_url() {
    // Though unusual, test handling of multiple hashes
    let line = "url: http://example.com#section1#subsection";

    assert!(!is_comment_line(line));

    let stripped = strip_inline_comment(line);
    assert!(stripped.contains('#'));

    let info = detect_mapping_key(line, 0);
    assert!(info.is_some());
    let info = info.unwrap();
    assert_eq!(info.value, Some("http://example.com#section1#subsection".to_string()));
}

#[test]
fn test_url_with_complex_anchor() {
    // URLs with complex anchor patterns
    let test_cases = vec![
        "url: https://example.com/api/v1#users/{id}",
        "doc: https://docs.example.com#section-1.2.3",
        "endpoint: http://localhost:3000/api#v2/list",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line));
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('#'));

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
    }
}

#[test]
fn test_url_with_port_and_anchor() {
    // URLs with port numbers and anchors
    let test_cases = vec![
        "url: http://example.com:8080#section",
        "api: https://api.example.com:443/v1#endpoint",
        "local: http://localhost:3000#home",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line));

        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('#'));
        assert!(stripped.contains(':'), "Port separator should be preserved in: {}", line);

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
    }
}

#[test]
fn test_url_only_hash_without_space_preserved() {
    // Hash in URL without preceding space is part of URL
    let test_cases = vec![
        "url: http://example.com#section",
        "api: https://api.com#endpoint",
        "link: ftp://files.com#dir",
    ];

    for line in test_cases {
        let stripped = strip_inline_comment(line);
        // The hash should be in the result (not stripped as comment)
        assert!(stripped.contains('#'), "Hash in URL should be preserved: {}", line);
    }
}

// YAML Anchors and Aliases Tests

#[test]
fn test_yaml_anchor_definition() {
    // YAML anchor definitions (&) should be detected as anchors, not comments
    let test_cases = vec![
        "&anchor_name",
        "  &anchor_name",
        "&my_anchor",
        "  &default_config",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line), "Anchor definition should not be comment: {}", line);
        assert_eq!(classify_line_type(line), LineType::Anchor);

        let stripped = strip_inline_comment(line);
        assert!(stripped.trim().starts_with('&'), "Anchor & should be preserved in: {}", line);
    }
}

#[test]
fn test_yaml_alias_reference() {
    // YAML alias references (*) should be detected as aliases, not comments
    let test_cases = vec![
        "*alias_name",
        "  *alias_name",
        "*my_alias",
        "  *default_config",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line), "Alias reference should not be comment: {}", line);
        assert_eq!(classify_line_type(line), LineType::Alias);

        let stripped = strip_inline_comment(line);
        assert!(stripped.trim().starts_with('*'), "Alias * should be preserved in: {}", line);
    }
}

#[test]
fn test_anchor_with_mapping_key() {
    // Anchors can be attached to mapping keys
    let test_cases = vec![
        "key: &anchor value",
        "config: &default_settings {host: localhost}",
        "database: &prod_db postgres://localhost/mydb",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line));
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('&'), "Anchor & should be preserved in: {}", line);

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
    }
}

#[test]
fn test_alias_as_mapping_value() {
    // Aliases can be used as values
    let test_cases = vec![
        "config: *default_settings",
        "database: *prod_db",
        "template: *base_template",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line));
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('*'), "Alias * should be preserved in: {}", line);

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert!(info.value.unwrap().starts_with('*'));
    }
}

#[test]
fn test_anchor_and_alias_not_confused_with_comment() {
    // Ensure & and * are never confused with #
    let test_cases = vec![
        "&anchor # this is a real comment",
        "*alias # this is a real comment",
    ];

    for line in test_cases {
        // The & and * part should not be comments
        let leading = line.split('#').next().unwrap();
        assert!(!is_comment_line(leading));

        let stripped = strip_inline_comment(line);
        // Should preserve & or * and strip the actual comment
        assert!(stripped.starts_with('&') || stripped.starts_with('*'));
        assert!(!stripped.contains('#') || stripped.ends_with(' '));
    }
}

#[test]
fn test_anchor_in_sequence() {
    // Anchors in sequence items
    let line = "- item: &anchor value";

    assert!(!is_comment_line(line));
    assert_eq!(classify_line_type(line), LineType::SequenceItem);

    let stripped = strip_inline_comment(line);
    assert!(stripped.contains('&'));
}

#[test]
fn test_alias_in_sequence() {
    // Aliases in sequence items
    let line = "- item: *alias_ref";

    assert!(!is_comment_line(line));
    assert_eq!(classify_line_type(line), LineType::SequenceItem);

    let stripped = strip_inline_comment(line);
    assert!(stripped.contains('*'));
}

// Tag Tests with Hash-like Patterns

#[test]
fn test_yaml_tag_definition() {
    // YAML tag definitions (!) should be detected as tags, not comments
    let test_cases = vec![
        "!tag",
        "  !tag",
        "!my_tag",
        "!custom_type",
        "  !MyCustomTag",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line), "Tag definition should not be comment: {}", line);
        assert_eq!(classify_line_type(line), LineType::Tag);

        let stripped = strip_inline_comment(line);
        assert!(stripped.trim().starts_with('!'), "Tag ! should be preserved in: {}", line);
    }
}

#[test]
fn test_tag_with_mapping_key() {
    // Tags can be attached to mapping keys
    let test_cases = vec![
        "key: !type value",
        "timestamp: !timestamp 2023-01-01T00:00:00Z",
        "data: !custom {field: value}",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line));
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('!'), "Tag ! should be preserved in: {}", line);

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
    }
}

#[test]
fn test_tag_not_confused_with_hash() {
    // Ensure ! is never confused with #
    let line = "key: !type value";

    assert!(!is_comment_line(line));
    assert_eq!(classify_line_type(line), LineType::MappingKey);

    let stripped = strip_inline_comment(line);
    assert!(stripped.contains('!'));
    assert!(!stripped.contains('#'));
}

#[test]
fn test_tag_with_inline_comment() {
    // Tags followed by inline comments
    let test_cases = vec![
        ("key: !type value # documentation", "key: !type value "),
        ("timestamp: !timestamp 2023-01-01 # ISO 8601", "timestamp: !timestamp 2023-01-01 "),
    ];

    for (line, expected_stripped) in test_cases {
        assert!(!is_comment_line(line));

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected_stripped);

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
    }
}

// Complex Real-world Scenarios

#[test]
fn test_realistic_config_file_with_hashes() {
    // Realistic configuration file with various hash patterns
    let yaml = r##"# Configuration file
app:
  theme: "#1a2b3c"  # Dark theme color
  url: https://example.com/api#v1
  separator: '#'
  anchor: &default_settings
    host: localhost
    port: 8080
  override: *default_settings
  types:
    - !CustomType value
    - !AnotherType reference
"##;

    let lines: Vec<&str> = yaml.lines().collect();

    // Verify comment lines
    assert!(is_comment_line(lines[0])); // # Configuration file

    // Verify hash in quoted values are preserved (not comments)
    // lines[2] is "theme: "#1a2b3c"  # Dark theme color" - mapping with inline comment
    assert!(!is_comment_line(lines[2])); // "#1a2b3c" is quoted color value
    let info = detect_mapping_key(lines[2], 2);
    assert!(info.is_some());
    assert!(info.unwrap().value.unwrap().contains('#'));

    // Verify URL anchor is preserved
    let info = detect_mapping_key(lines[3], 2);
    assert!(info.is_some());
    assert!(info.unwrap().value.unwrap().contains('#'));

    // Verify separator with quoted hash
    assert!(!is_comment_line(lines[4])); // separator: '#'
    let info = detect_mapping_key(lines[4], 2);
    assert!(info.is_some());

    // Verify anchor with mapping key (line is "anchor: &default_settings")
    assert!(!is_comment_line(lines[5]));
    assert_eq!(classify_line_type(lines[5]), LineType::MappingKey);
    let stripped = strip_inline_comment(lines[5]);
    assert!(stripped.contains('&'));
    let info = detect_mapping_key(lines[5], 2);
    assert!(info.is_some());

    // Verify alias reference
    assert!(!is_comment_line(lines[8])); // *default_settings
    let info = detect_mapping_key(lines[8], 2);
    assert!(info.is_some());
    assert!(info.unwrap().value.unwrap().starts_with('*'));

    // Verify tags in sequence
    assert_eq!(classify_line_type(lines[10]), LineType::SequenceItem);
    assert!(strip_inline_comment(lines[10]).contains('!'));
}

#[test]
fn test_css_like_configuration() {
    // CSS-like configuration: unquoted hex colors have # treated as comment
    // To preserve hex colors, they MUST be quoted
    let test_cases = vec![
        ("background: #FFFFFF", "background: "), // Unquoted # is comment
        ("foreground: #000000", "foreground: "),
        ("accent: #ff6600", "accent: "),
        ("transparent: #00000000", "transparent: "),
    ];

    for (line, expected_stripped) in test_cases {
        assert!(!is_comment_line(line)); // Line has content before #
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected_stripped); // # is stripped as comment

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
        let info = info.unwrap();
        // Value is None or empty since # starts comment
        assert!(info.value.is_none() || info.value.unwrap().trim().is_empty());
    }
}

#[test]
fn test_documentation_urls_with_anchors() {
    // Documentation URLs with section anchors
    let yaml = r#"docs:
  api: https://docs.example.com/api#authentication
  guide: https://docs.example.com/guide#getting-started
  reference: https://docs.example.com/ref#types
"#;

    let lines: Vec<&str> = yaml.lines().collect();

    for line in &lines[1..] { // Skip "docs:" line
        if !line.trim().is_empty() {
            assert!(!is_comment_line(line));

            let info = detect_mapping_key(line, 2);
            assert!(info.is_some());
            let info = info.unwrap();
            assert!(info.value.unwrap().contains('#'));
        }
    }
}

#[test]
fn test_hash_in_various_positions() {
    // Test hash in various positions within values (NOT preceded by whitespace)
    let test_cases = vec![
        ("middle: value#middle", "middle", "value#middle"),
        ("end: value#", "end", "value#"),
        ("multiple: val#ue#hash", "multiple", "val#ue#hash"),
        ("concat: a#b#c", "concat", "a#b#c"),
    ];

    for (line, expected_key, expected_value) in test_cases {
        assert!(!is_comment_line(line));

        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('#'));

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, expected_key);
        assert_eq!(info.value, Some(expected_value.to_string()));
    }
}


#[test]
fn test_mixed_special_characters() {
    // Mix of special characters including hash
    let test_cases = vec![
        "symbol1: $value#hash",
        "symbol2: @value#hash",
        "symbol3: %value#hash",
        "symbol4: &value#hash",
        "symbol5: *value#hash",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line));

        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('#'));

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
    }
}

#[test]
fn test_inline_comment_after_hash_value() {
    // Quoted hash values followed by actual inline comments
    let test_cases = vec![
        ("color: \"#FFF\" # white color", "color: \"#FFF\" "),
        ("url: https://example.com#anchor # docs", "url: https://example.com#anchor "),
        ("value: test#notcomment # this is comment", "value: test#notcomment "),
    ];

    for (line, expected_stripped) in test_cases {
        assert!(!is_comment_line(line));

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected_stripped);

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
    }
}

#[test]
fn test_hash_without_whitespace_is_value() {
    // Hash without preceding whitespace is always part of value
    let test_cases = vec![
        "key: value#notcomment",
        "config: setting#enabled",
        "path: /some/path#fragment",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line));

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, line);

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert!(info.value.unwrap().contains('#'));
    }
}

#[test]
fn test_hash_with_whitespace_is_comment() {
    // Hash with preceding whitespace starts a comment
    let test_cases = vec![
        ("key: value # this is comment", "key: value "),
        ("setting: enabled # documentation", "setting: enabled "),
        ("path: /path/to/file # file path", "path: /path/to/file "),
    ];

    for (line, expected_stripped) in test_cases {
        assert!(!is_comment_line(line));

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected_stripped);

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
        let info = info.unwrap();
        // Value should not contain the comment part
        assert!(!info.value.unwrap().contains('#'));
    }
}

// Additional Tag Tests with Hash-like Patterns

#[test]
fn test_tag_with_hash_like_patterns() {
    // Tags containing hash-like patterns (text "hash" in tag name)
    let test_cases = vec![
        "type: !hash_map",
        "kind: !hash_set",
        "format: !hash_ref",
        "struct: !Hash_Map",
        "collection: !Hash_Set_Struct",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line), "Tag with hash-like pattern should not be comment: {}", line);
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('!'), "Tag ! should be preserved in: {}", line);
        assert!(stripped.contains("hash") || stripped.contains("Hash"), "Hash text in tag should be preserved in: {}", line);

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
    }
}

#[test]
fn test_tag_with_hash_symbol_in_name() {
    // Tags with actual # symbol in the name (edge case)
    let test_cases = vec![
        "type: !hash#type",
        "kind: !module#CustomType",
        "format: !encoder#base64",
        "struct: !my_pkg#Hash#Struct",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line), "Tag with # in name should not be comment: {}", line);
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('!'), "Tag ! should be preserved in: {}", line);
        assert!(stripped.contains('#'), "# in tag name should be preserved in: {}", line);

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
    }
}

#[test]
fn test_tag_with_hash_like_pattern_and_comment() {
    // Tags with hash-like patterns followed by inline comments
    let test_cases = vec![
        ("type: !hash_map value # custom type", "type: !hash_map value "),
        ("kind: !hash_set data # set type", "kind: !hash_set data "),
        ("struct: !Hash_Struct obj # hash struct", "struct: !Hash_Struct obj "),
    ];

    for (line, expected_stripped) in test_cases {
        assert!(!is_comment_line(line));

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected_stripped);

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert!(info.value.as_ref().unwrap().contains('!'));
    }
}

#[test]
fn test_tag_with_hash_in_name_and_comment() {
    // Tags with # in name followed by inline comments
    let test_cases = vec![
        ("type: !hash#type value # custom hash type", "type: !hash#type value "),
        ("kind: !mod#Custom data # custom module", "kind: !mod#Custom data "),
    ];

    for (line, expected_stripped) in test_cases {
        assert!(!is_comment_line(line));

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected_stripped);

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
        let info = info.unwrap();
        // Value should preserve tag with # but strip comment
        assert!(info.value.as_ref().unwrap().contains('!'));
        assert!(info.value.as_ref().unwrap().contains('#'));
    }
}

#[test]
fn test_complex_tag_patterns_with_hash() {
    // Complex tag patterns involving hash
    let test_cases = vec![
        "type: !!my_module#HashType value",
        "kind: !local!hash#map data",
        "format: !tag:example.com,2014:hash#type value",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line), "Complex tag pattern should not be comment: {}", line);
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('!'), "Tag ! should be preserved in: {}", line);

        let info = detect_mapping_key(line, 0);
        assert!(info.is_some());
    }
}
