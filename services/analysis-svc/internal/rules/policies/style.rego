package style

# Unused Variable Detection
unused_variable[result] {
    assignment := ast.assignment
    assignment.target = var_name
    not variable_used(var_name)
    
    result := {
        "rule_id": "style.unused_variable",
        "severity": "low",
        "message": sprintf("Variable %q assigned but never used", [var_name]),
        "file_path": input.file_path,
        "start_line": assignment.start_line,
        "end_line": assignment.end_line,
        "start_column": assignment.start_column,
        "end_column": assignment.end_column,
        "code_snippet": input.source[assignment.start_byte:assignment.end_byte]
    }
}

variable_used(var_name) {
    usage := ast.variable_usage
    usage.name == var_name
}

variable_used(var_name) {
    usage := ast.attribute_access
    usage.object.name == var_name
}

# Trailing Whitespace
trailing_whitespace[result] {
    line := input.lines[_]
    endswith(line, " ")
    
    result := {
        "rule_id": "style.trailing_whitespace",
        "severity": "info",
        "message": "Trailing whitespace",
        "file_path": input.file_path,
        "start_line": line.number,
        "end_line": line.number,
        "start_column": strlen(line) - strlen(trim(line, " ")) + 1,
        "end_column": strlen(line) + 1,
        "code_snippet": line
    }
}

# Missing Docstring for Public Functions
missing_docstring[result] {
    func := ast.function_definition
    func.is_public
    not has_docstring(func)
    
    result := {
        "rule_id": "style.missing_docstring",
        "severity": "low",
        "message": sprintf("Public function %q missing docstring", [func.name]),
        "file_path": input.file_path,
        "start_line": func.start_line,
        "end_line": func.start_line,
        "start_column": 1,
        "end_column": func.end_column,
        "code_snippet": input.source[func.start_byte:func.end_byte]
    }
}

has_docstring(func) {
    func.body[0].type == "string"
}

# Too Many Arguments
too_many_arguments[result] {
    func := ast.function_definition
    count(func.parameters) > 7
    
    result := {
        "rule_id": "style.too_many_arguments",
        "severity": "medium",
        "message": sprintf("Function %q has %d arguments (max 7)", [func.name, count(func.parameters)]),
        "file_path": input.file_path,
        "start_line": func.start_line,
        "end_line": func.end_line,
        "start_column": 1,
        "end_column": func.end_column,
        "code_snippet": input.source[func.start_byte:func.end_byte]
    }
}

# Long Function
long_function[result] {
    func := ast.function_definition
    func.end_line - func.start_line > 50
    
    result := {
        "rule_id": "style.long_function",
        "severity": "medium",
        "message": sprintf("Function %q is %d lines long (max 50)", [func.name, func.end_line - func.start_line]),
        "file_path": input.file_path,
        "start_line": func.start_line,
        "end_line": func.end_line,
        "start_column": 1,
        "end_column": func.end_column,
        "code_snippet": input.source[func.start_byte:func.end_byte]
    }
}

# Nested Too Deeply
nested_too_deeply[result] {
    block := ast.block
    block.nesting_level > 4
    
    result := {
        "rule_id": "style.nested_too_deeply",
        "severity": "medium",
        "message": sprintf("Code nested %d levels deep (max 4)", [block.nesting_level]),
        "file_path": input.file_path,
        "start_line": block.start_line,
        "end_line": block.end_line,
        "start_column": 1,
        "end_column": block.end_column,
        "code_snippet": input.source[block.start_byte:block.end_byte]
    }
}

# Magic Numbers
magic_number[result] {
    literal := ast.number_literal
    literal.value not in [0, 1, -1, 2, 10, 100]
    not in_constant_assignment(literal)
    not in_loop_range(literal)
    
    result := {
        "rule_id": "style.magic_number",
        "severity": "low",
        "message": sprintf("Magic number %v should be a named constant", [literal.value]),
        "file_path": input.file_path,
        "start_line": literal.start_line,
        "end_line": literal.end_line,
        "start_column": literal.start_column,
        "end_column": literal.end_column,
        "code_snippet": sprintf("%v", [literal.value])
    }
}

in_constant_assignment(literal) {
    parent := literal.parent
    parent.type == "assignment"
    parent.target.matches("^[A-Z_]+$")
}

in_loop_range(literal) {
    parent := literal.parent
    parent.type == "call"
    parent.function_name == "range"
}