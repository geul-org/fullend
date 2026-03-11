@scenario
Feature: Gig lifecycle happy path

  Scenario: Client creates gig, freelancer submits proposal, client accepts, freelancer submits work, client approves
    Given POST Register {"Email": "client-lifecycle@test.com", "Password": "Pass1234!", "Role": "client", "Name": "Client User"} → clientUser
    And POST Login {"Email": "client-lifecycle@test.com", "Password": "Pass1234!"} → token
    And POST CreateGig {"Title": "Build API", "Description": "Build a REST API", "Budget": 100000} → gig
    And PUT PublishGig {"ID": gig.ID} → gig
    Then status == 200
    And response.gig.status == "open"

    Given POST Register {"Email": "freelancer-lifecycle@test.com", "Password": "Pass1234!", "Role": "freelancer", "Name": "Freelancer A"} → freelancerUser
    And POST Login {"Email": "freelancer-lifecycle@test.com", "Password": "Pass1234!"} → token
    And POST SubmitProposal {"ID": gig.ID, "BidAmount": 90000} → proposal
    Then status == 200

    Given POST Login {"Email": "client-lifecycle@test.com", "Password": "Pass1234!"} → token
    When POST AcceptProposal {"ID": proposal.ID} → acceptResult
    Then status == 200
    And response.transactionID exists

    Given POST Login {"Email": "freelancer-lifecycle@test.com", "Password": "Pass1234!"} → token
    When POST SubmitWork {"ID": gig.ID} → gig
    Then status == 200
    And response.gig.status == "under_review"

    Given POST Login {"Email": "client-lifecycle@test.com", "Password": "Pass1234!"} → token
    When POST ApproveWork {"ID": gig.ID} → approveResult
    Then status == 200
    And response.gig.status == "completed"
    And response.transactionID exists

@invariant
Feature: Unauthorized freelancer cannot submit work on another freelancers gig

  Scenario: Freelancer B tries to submit work on Freelancer A assigned gig
    Given POST Register {"Email": "client-inv1@test.com", "Password": "Pass1234!", "Role": "client", "Name": "Client"} → clientUser
    And POST Login {"Email": "client-inv1@test.com", "Password": "Pass1234!"} → token
    And POST CreateGig {"Title": "Inv Test Gig", "Description": "Test", "Budget": 50000} → gig
    And PUT PublishGig {"ID": gig.ID}

    Given POST Register {"Email": "freelancerA-inv1@test.com", "Password": "Pass1234!", "Role": "freelancer", "Name": "Freelancer A"} → freelancerA
    And POST Login {"Email": "freelancerA-inv1@test.com", "Password": "Pass1234!"} → token
    And POST SubmitProposal {"ID": gig.ID, "BidAmount": 40000} → proposal

    Given POST Login {"Email": "client-inv1@test.com", "Password": "Pass1234!"} → token
    And POST AcceptProposal {"ID": proposal.ID}

    Given POST Register {"Email": "freelancerB-inv1@test.com", "Password": "Pass1234!", "Role": "freelancer", "Name": "Freelancer B"} → freelancerB
    And POST Login {"Email": "freelancerB-inv1@test.com", "Password": "Pass1234!"} → token
    When POST SubmitWork {"ID": gig.ID}
    Then status == 403

@invariant
Feature: Cannot approve work when gig is in open state

  Scenario: Client tries to approve work on an open gig
    Given POST Register {"Email": "client-inv2@test.com", "Password": "Pass1234!", "Role": "client", "Name": "Client"} → clientUser
    And POST Login {"Email": "client-inv2@test.com", "Password": "Pass1234!"} → token
    And POST CreateGig {"Title": "State Test Gig", "Description": "Test", "Budget": 30000} → gig
    And PUT PublishGig {"ID": gig.ID}
    When POST ApproveWork {"ID": gig.ID}
    Then status == 409
