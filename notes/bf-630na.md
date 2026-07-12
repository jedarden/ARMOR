# Update Validate() Callers to Handle YAMLError (bf-630na)

## Summary

Updated `TestSchemaDefinition_Validate_Contract` in `internal/yamlutil/schema_validation_test.go` to properly handle YAMLError return types from `Compile()` method calls.

## Changes Made

### 1. Updated Error Messages
- Changed error messages from "Validate()" to "Compile()" to accurately reflect the method being tested
- Lines 98, 105, 113: Updated error messages to reference Compile() instead of Validate()

### 2. Enhanced YAMLError Type Checking
- Added detailed YAMLError logging for debugging purposes
- When YAMLError is detected, logs: Code, Type, and Context
- Helps with debugging schema validation failures

### 3. Updated Test Documentation
- Fixed test comment to accurately describe what's being tested
- Changed from "Validate() method returns YAMLError-compatible types" to "Compile() method returns YAMLError-compatible types for schema errors"

## Test Results

All schema validation tests pass successfully:
- ✅ TestSchemaDefinition_Validate_Contract
- ✅ TestSchemaDefinition_Interface  
- ✅ TestSchemaDefinition_Validate_GenericValues
- ✅ TestSchemaDefinition_Validate_NestedStructures
- ✅ TestSchemaDefinition_Name_Version_Description
- ✅ TestSchemaDefinition_ValidateFile
- ✅ TestSchemaValidationResult_ErrorIntegration

## YAMLError Details Logged

The enhanced test now logs YAMLError details including:
- **Code**: Error code (e.g., SCHEMA_INVALID, CONSTRAINT_VIOLATION)
- **Type**: Error type (e.g., schema_load, schema_validate)
- **Context**: Additional context about the error

This preserves debugging context while properly handling the YAMLError interface type.
