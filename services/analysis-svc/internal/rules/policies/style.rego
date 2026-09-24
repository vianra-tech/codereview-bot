package style

import rego.v1

# Unused Variable Detection
unused_variable contains result if {
    assignment := input.ast.assignment
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
        "code_snippet": substring(input.source, assignment.start_byte, assignment.end_byte - assignment.start_byte),
    }
}

variable_used(var_name) if {
    usage := input.ast.variable_usage
    usage.name == var_name
}

variable_used(var_name) if {
    usage := input.ast.attribute_access
    usage.object.name == var_name
}

# Trailing Whitespace
trailing_whitespace contains result if {
    line := input.lines[_]
    endswith(line.content, " ")

    result := {
        "rule_id": "style.trailing_whitespace",
        "severity": "info",
        "message": "Trailing whitespace",
        "file_path": input.file_path,
        "start_line": line.number,
        "end_line": line.number,
        "start_column": count(line.content) - count(trim_right(line.content, " ")) + 1,
        "end_column": count(line.content) + 1,
        "code_snippet": line.content,
    }
}

# Missing Docstring for Public Functions
missing_docstring contains result if {
    func := input.ast.function_definition
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
        "code_snippet": substring(input.source, func.start_byte, func.end_byte - func.start_byte),
    }
}

has_docstring(func) if {
    func.body[0].type == "string"
}

# Too Many Arguments
too_many_arguments contains result if {
    func := input.ast.function_definition
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
        "code_snippet": substring(input.source, func.start_byte, func.end_byte - func.start_byte),
    }
}

# Long Function
long_function contains result if {
    func := input.ast.function_definition
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
        "code_snippet": substring(input.source, func.start_byte, func.end_byte - func.start_byte),
    }
}

# Nested Too Deeply
nested_too_deeply contains result if {
    block := input.ast.block
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
        "code_snippet": substring(input.source, block.start_byte, block.end_byte - block.start_byte),
    }
}

# Magic Numbers
magic_number contains result if {
    literal := input.ast.number_literal
    not literal.value in [0, 1, -1, 2, 10, 100]
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
        "code_snippet": sprintf("%v", [literal.value]),
    }
}

in_constant_assignment(literal) if {
    parent := literal.parent
    parent.type == "assignment"
    regex.match("^[A-Z_]+$", parent.target)
}

in_loop_range(literal) if {
    parent := literal.parent
    parent.type == "call"
    parent.function_name == "range"
}