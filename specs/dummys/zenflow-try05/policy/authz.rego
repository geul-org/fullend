package authz

# @ownership workflow: workflows.org_id

default allow = false

allow if {
    input.action == "CreateWorkflow"
    input.resource == "workflow"
    input.claims.role == "admin"
}

allow if {
    input.action == "ListWorkflows"
    input.resource == "workflow"
}

allow if {
    input.action == "ActivateWorkflow"
    input.resource == "workflow"
    input.claims.role == "admin"
}

allow if {
    input.action == "PauseWorkflow"
    input.resource == "workflow"
    input.claims.role == "admin"
}

allow if {
    input.action == "ArchiveWorkflow"
    input.resource == "workflow"
    input.claims.role == "admin"
}

allow if {
    input.action == "AddAction"
    input.resource == "workflow"
    input.claims.role == "admin"
}

allow if {
    input.action == "ExecuteWorkflow"
    input.resource == "workflow"
}

allow if {
    input.action == "ListExecutionLogs"
    input.resource == "execution_log"
}
