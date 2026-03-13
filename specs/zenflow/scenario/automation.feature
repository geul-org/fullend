Feature: Workflow Automation

  @scenario
  Scenario: Happy Path - Create and Execute Workflow
    When POST CreateOrganization {"name":"Acme Corp","plan_type":"pro","credits_balance":100} → org
    Then status == 200

    When POST Register {"org_id": org.organization.id, "email":"admin@acme.com","password":"pass123","role":"admin"}
    Then status == 200

    When POST Login {"email":"admin@acme.com","password":"pass123"} → adminToken
    Then status == 200

    When POST CreateWorkflow {"title":"Email Campaign","trigger_event":"webhook_received"} → wf
    Then status == 200
    Then response.workflow.status == "draft"

    When POST CreateAction {"id": wf.workflow.id, "action_type":"send_email","sequence_order":1}
    Then status == 200
    When POST CreateAction {"id": wf.workflow.id, "action_type":"http_request","sequence_order":2}
    Then status == 200

    When PUT ActivateWorkflow {"id": wf.workflow.id}
    Then status == 200
    Then response.workflow.status == "active"

    When POST ExecuteWorkflow {"id": wf.workflow.id}
    Then status == 200
    Then response.log exists

  @invariant
  Scenario: Tenant Breach - Org A user cannot access Org B workflow
    When POST CreateOrganization {"name":"Org A","plan_type":"pro","credits_balance":100} → orgA
    Then status == 200
    When POST CreateOrganization {"name":"Org B","plan_type":"pro","credits_balance":100} → orgB
    Then status == 200

    When POST Register {"org_id": orgA.organization.id, "email":"adminA@test.com","password":"pass123","role":"admin"}
    Then status == 200
    When POST Register {"org_id": orgB.organization.id, "email":"adminB@test.com","password":"pass123","role":"admin"}
    Then status == 200

    When POST Login {"email":"adminB@test.com","password":"pass123"} → tokenB
    Then status == 200
    When POST CreateWorkflow {"title":"B Workflow","trigger_event":"cron"} → wfB
    Then status == 200

    When POST Login {"email":"adminA@test.com","password":"pass123"} → tokenA
    Then status == 200
    When PUT ActivateWorkflow {"id": wfB.workflow.id}
    Then status == 403

  @invariant
  Scenario: Insufficient Credits - Cannot activate with zero credits
    When POST CreateOrganization {"name":"Broke Corp","plan_type":"free","credits_balance":0} → brokeOrg
    Then status == 200
    When POST Register {"org_id": brokeOrg.organization.id, "email":"broke@test.com","password":"pass123","role":"admin"}
    Then status == 200
    When POST Login {"email":"broke@test.com","password":"pass123"} → brokeToken
    Then status == 200
    When POST CreateWorkflow {"title":"Broke Workflow","trigger_event":"manual"} → brokeWf
    Then status == 200
    When PUT ActivateWorkflow {"id": brokeWf.workflow.id}
    Then status == 404
