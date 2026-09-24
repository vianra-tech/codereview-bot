package security

import rego.v1

# SQL Injection Detection
# Detects string concatenation in SQL queries
sql_injection contains result if {
    # Look for call expressions that might be SQL execution
    call := input.ast.call_expr
    call.function_name == "execute"

    # Check if the query is built via string concatenation
    call.arguments[_] = concat_expr
    concat_expr.type == "binary_operator"
    concat_expr.operator == "+"

    result := {
        "rule_id": "security.sql_injection",
        "severity": "critical",
        "message": "Potential SQL injection via string concatenation",
        "file_path": input.file_path,
        "start_line": call.start_line,
        "end_line": call.end_line,
        "start_column": call.start_column,
        "end_column": call.end_column,
        "code_snippet": substring(input.source, call.start_byte, call.end_byte - call.start_byte),
    }
}

# Also detect .format() usage in SQL
sql_injection contains result if {
    call := input.ast.call_expr
    call.function_name == "format"
    call.object.value == "SELECT"

    result := {
        "rule_id": "security.sql_injection",
        "severity": "critical",
        "message": "Potential SQL injection via .format()",
        "file_path": input.file_path,
        "start_line": call.start_line,
        "end_line": call.end_line,
        "start_column": call.start_column,
        "end_column": call.end_column,
        "code_snippet": substring(input.source, call.start_byte, call.end_byte - call.start_byte),
    }
}

# Command Injection Detection
command_injection contains result if {
    call := input.ast.call_expr
    call.function_name in ["system", "popen", "subprocess.run", "subprocess.Popen", "os.system"]

    # Check if arguments contain user input
    call.arguments[_] = arg
    contains_user_input(arg)

    result := {
        "rule_id": "security.command_injection",
        "severity": "critical",
        "message": "Potential command injection",
        "file_path": input.file_path,
        "start_line": call.start_line,
        "end_line": call.end_line,
        "start_column": call.start_column,
        "end_column": call.end_column,
        "code_snippet": substring(input.source, call.start_byte, call.end_byte - call.start_byte),
    }
}

contains_user_input(expr) if {
    expr.type == "variable"
    expr.name in ["request", "input", "params", "query", "body", "data"]
}

contains_user_input(expr) if {
    expr.type == "attribute"
    expr.object.name in ["request", "input", "params", "query", "body", "data"]
}

# Path Traversal Detection
path_traversal contains result if {
    call := input.ast.call_expr
    call.function_name in ["open", "file", "readFile", "writeFile"]

    call.arguments[_] = arg
    contains_path_traversal(arg)

    result := {
        "rule_id": "security.path_traversal",
        "severity": "high",
        "message": "Potential path traversal",
        "file_path": input.file_path,
        "start_line": call.start_line,
        "end_line": call.end_line,
        "start_column": call.start_column,
        "end_column": call.end_column,
        "code_snippet": substring(input.source, call.start_byte, call.end_byte - call.start_byte),
    }
}

contains_path_traversal(expr) if {
    expr.type == "binary_operator"
    expr.operator == "+"
    contains_user_input(expr.left)
    expr.right.value == ".."
}

contains_path_traversal(expr) if {
    expr.type == "call"
    expr.function_name == "join"
    expr.arguments[_] = arg
    contains_user_input(arg)
}

# Hardcoded Secret Detection
hardcoded_secret contains result if {
    assignment := input.ast.assignment
    regex.match("(?i)(password|secret|token|key|api_key|apikey)", assignment.target)
    assignment.value.type == "string"

    result := {
        "rule_id": "security.hardcoded_secret",
        "severity": "critical",
        "message": "Hardcoded secret detected",
        "file_path": input.file_path,
        "start_line": assignment.start_line,
        "end_line": assignment.end_line,
        "start_column": assignment.start_column,
        "end_column": assignment.end_column,
        "code_snippet": substring(input.source, assignment.start_byte, assignment.end_byte - assignment.start_byte),
    }
}

# XSS Detection
xss contains result if {
    call := input.ast.call_expr
    call.function_name in ["render_template_string", "Markup", "safe", "dangerouslySetInnerHTML"]

    result := {
        "rule_id": "security.xss",
        "severity": "high",
        "message": "Potential XSS - user input rendered without escaping",
        "file_path": input.file_path,
        "start_line": call.start_line,
        "end_line": call.end_line,
        "start_column": call.start_column,
        "end_column": call.end_column,
        "code_snippet": substring(input.source, call.start_byte, call.end_byte - call.start_byte),
    }
}

# Generic user input detection for taint tracking
user_input contains var if {
    assignment := input.ast.assignment
    assignment.target = var
    assignment.value.type == "call"
    assignment.value.function_name in ["request.args.get", "request.form.get", "request.json.get", "input"]
}