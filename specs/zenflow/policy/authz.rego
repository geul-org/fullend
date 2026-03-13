package authz

# @ownership workflow_org: workflows.org_id
# @ownership user_org: users.org_id

default allow = false

allow if {
    input.action == "CreateWorkflow"
    input.resource == "user_org"
    input.claims.role == "admin"
}

allow if {
    input.action == "GetWorkflow"
    input.resource == "workflow_org"
    data.owners.workflow_org[input.resource_id] == data.owners.user_org[input.claims.user_id]
}

allow if {
    input.action == "CreateAction"
    input.resource == "workflow_org"
    input.claims.role == "admin"
    data.owners.workflow_org[input.resource_id] == data.owners.user_org[input.claims.user_id]
}

allow if {
    input.action == "ActivateWorkflow"
    input.resource == "workflow_org"
    input.claims.role == "admin"
    data.owners.workflow_org[input.resource_id] == data.owners.user_org[input.claims.user_id]
}

allow if {
    input.action == "PauseWorkflow"
    input.resource == "workflow_org"
    input.claims.role == "admin"
    data.owners.workflow_org[input.resource_id] == data.owners.user_org[input.claims.user_id]
}

allow if {
    input.action == "ArchiveWorkflow"
    input.resource == "workflow_org"
    input.claims.role == "admin"
    data.owners.workflow_org[input.resource_id] == data.owners.user_org[input.claims.user_id]
}

allow if {
    input.action == "ExecuteWorkflow"
    input.resource == "workflow_org"
    data.owners.workflow_org[input.resource_id] == data.owners.user_org[input.claims.user_id]
}
