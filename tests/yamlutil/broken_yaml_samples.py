"""
Broken YAML Sample Files for Comprehensive Testing

This module contains a collection of YAML samples that demonstrate various
syntax errors to test the validation layer's ability to detect and categorize
different types of issues.
"""

# Indentation Errors
INDENTATION_MIXED_SPACES_TABS = """
key: value
	nested_key: value
	deep:
	  mixed: indentation
"""

INDENTATION_INCONSISTENT = """
parent:
  child1: value
    child2: value  # Too indented
  child3: value
"""

INDENTATION_UNEXPECTED = """
root:
  level1: value
   level2: value  # Inconsistent indentation
"""

# Delimiter Errors
DELIMITER_MISSING_COLON = """
key value
another: correct
"""

DELIMITER_INVALID_COLON = """
key: value: bad: syntax
flow: {a: b: c}
"""

DELIMITER_UNCLOSED_QUOTE = """
single: 'unclosed string
double: "also unclosed
"""

# Structure Errors
STRUCTURE_DUPLICATE_KEY = """
mapping:
  key: value1
  key: value2
"""

STRUCTURE_INVALID_SEQUENCE = """
sequence:
  - item1
  - item2
  key: value  # Can't mix sequence items with mapping keys
"""

STRUCTURE_NESTED_FLOW = """
invalid: {key: {subkey: value}
unclosed: [item1, item2
"""

# Flow Collection Errors
FLOW_UNCLOSED_BRACE = """
mapping: {key: value, another: item
"""

FLOW_UNCLOSED_BRACKET = """
list: [item1, item2, item3
"""

FLOW_MISMATCHED_BRACES = """
mixed: {key: [item1, item2}}
"""

FLOW_INVALID_COMMA = """
bad: {key1 value1, key2: value2}
"""

# Scalar Errors
SCALAR_UNCLOSED_SINGLE_QUOTE = """
key: 'this string never closes
next: value
"""

SCALAR_UNCLOSED_DOUBLE_QUOTE = """
key: "this string never closes
next: value
"""

SCALAR_INVALID_ESCAPE = """
key: "invalid escape \\x sequence"
"""

SCALAR_MULTILINE_ISSUE = """
key: >
  this is a multiline scalar
    but indentation is wrong
next: value
"""

# Tag Errors
TAG_INVALID = """
key: !invalidtag value
another: !!python/object:invalid
"""

TAG_UNKNOWN = """
custom: !CustomTag
  field: value
"""

# Anchor/Alias Errors
ANCHOR_UNDEFINED_ALIAS = """
defined: &anchor value
reference: *undefined_alias
"""

ANCHOR_CIRCULAR = """
item: &self
  ref: *self
"""

ANCHOR_MULTIPLE_USE = """
key: &anchor value
ref1: *anchor
ref2: *anchor
"""

# Document Errors
DOCUMENT_MULTIPLE_WITHOUT_SEPARATOR = """
---
doc1: value
doc2: value
"""

DOCUMENT_INVALID_DIRECTIVES = """
%INVALID 123
%YAML 1.2
key: value
"""

DOCUMENT_EMPTY_STREAM = """
# Just comments
# No actual document
"""

# Complex Mixed Errors
COMPLEX_INDENTATION_FLOW = """
config:
  database:
    host: localhost
    port: 5432
    credentials: {user: admin, pass: secret
  cache:
    enabled: true
"""

COMPLEX_NESTED_SCALARS = """
services:
  api:
    config: |
      {
        "setting": "value,
        "nested": {
          "key": unclosed
      }
    }
  database:
    url: 'postgres://localhost:5432/db
"""

# Edge Cases
EDGE_TAB_IN_COMMENT = """
# This comment has a\ttab
key: value
"""

EDGE_TRAILING_SPACES = """key: value
another: value\t
"""

EDGE_EMPTY_LINES = """


key: value


another: value


"""

EDGE_SPECIAL_CHARACTERS = """
key: "value with special: characters"
another: [value, with, commas]
flow: "{key: value}"
"""

# Real-world Config Errors
REAL_WORLD_K8S_MISSING_COLON = """
apiVersion: v1
kind ConfigMap  # Missing colon
metadata:
  name test-config
data:
  key value
"""

REAL_WORLD_DOCKER_INVALID_YAML = """
version: '3.8'
services:
  web:
    image: nginx:latest
    ports:
      - "80:80
    volumes:
      - ./html:/usr/share/nginx/html
    depends_on:
      - backend
  backend:
    image: python:3.9
    command: python app.py
    environment:
      DEBUG: "true"
      DATABASE_URL: postgresql://localhost/db
"""
