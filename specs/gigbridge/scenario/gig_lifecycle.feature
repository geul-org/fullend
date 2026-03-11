Feature: Gig Lifecycle

  @scenario
  Scenario: Happy Path - Gig creation to fund release
    Given POST Register {"email": "client@test.com", "password": "pass123", "role": "client", "name": "Client A"}
    Then status == 200

    When POST Login {"email": "client@test.com", "password": "pass123"} -> clientToken
    Then status == 200

    When POST CreateGig {"title": "Build API", "description": "REST API project", "budget": 1000} -> gigResult
    Then status == 200

    When PUT PublishGig {"id": gigResult.gig.id}
    Then status == 200

    Given POST Register {"email": "freelancer@test.com", "password": "pass123", "role": "freelancer", "name": "Freelancer A"}
    Then status == 200

    When POST Login {"email": "freelancer@test.com", "password": "pass123"} -> freelancerToken
    Then status == 200

    When POST SubmitProposal {"id": gigResult.gig.id, "bid_amount": 900} -> proposalResult
    Then status == 200

    When POST AcceptProposal {"id": proposalResult.proposal.id}
    Then status == 200

    When POST SubmitWork {"id": gigResult.gig.id}
    Then status == 200

    When POST ApproveWork {"id": gigResult.gig.id}
    Then status == 200
    Then response.gig.status == "completed"

  @invariant
  Scenario: Unauthorized - Freelancer B cannot submit work on Freelancer A's gig
    Given POST Register {"email": "client2@test.com", "password": "pass123", "role": "client", "name": "Client B"}
    Then status == 200

    When POST Login {"email": "client2@test.com", "password": "pass123"} -> clientToken2
    Then status == 200

    When POST CreateGig {"title": "Design Logo", "description": "Logo design", "budget": 500} -> gig2
    Then status == 200

    When PUT PublishGig {"id": gig2.gig.id}
    Then status == 200

    Given POST Register {"email": "freelancerA@test.com", "password": "pass123", "role": "freelancer", "name": "Freelancer A2"}
    Then status == 200

    When POST Login {"email": "freelancerA@test.com", "password": "pass123"} -> flTokenA
    Then status == 200

    When POST SubmitProposal {"id": gig2.gig.id, "bid_amount": 400} -> proposal2
    Then status == 200

    When POST AcceptProposal {"id": proposal2.proposal.id}
    Then status == 200

    Given POST Register {"email": "freelancerB@test.com", "password": "pass123", "role": "freelancer", "name": "Freelancer B"}
    Then status == 200

    When POST Login {"email": "freelancerB@test.com", "password": "pass123"} -> flTokenB
    Then status == 200

    When POST SubmitWork {"id": gig2.gig.id}
    Then status == 403

  @invariant
  Scenario: Invalid State - Cannot approve work when gig is open
    Given POST Register {"email": "client3@test.com", "password": "pass123", "role": "client", "name": "Client C"}
    Then status == 200

    When POST Login {"email": "client3@test.com", "password": "pass123"} -> clientToken3
    Then status == 200

    When POST CreateGig {"title": "Write Docs", "description": "Documentation", "budget": 300} -> gig3
    Then status == 200

    When PUT PublishGig {"id": gig3.gig.id}
    Then status == 200

    When POST ApproveWork {"id": gig3.gig.id}
    Then status == 409
