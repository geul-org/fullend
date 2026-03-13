Feature: Gig Lifecycle

  @scenario
  Scenario: Happy Path - Full Gig Lifecycle
    When POST Register {"email":"client@test.com","password":"pass123","role":"client","name":"Test Client"}
    Then status == 200
    When POST Register {"email":"freelancerA@test.com","password":"pass123","role":"freelancer","name":"Freelancer A"}
    Then status == 200

    When POST Login {"email":"client@test.com","password":"pass123"} → clientToken
    Then status == 200
    When POST CreateGig {"title":"Build Website","description":"Need a website built","budget":5000} → gig
    Then status == 200
    Then response.gig.status == "draft"
    When PUT PublishGig {"id": gig.gig.id}
    Then status == 200
    Then response.gig.status == "open"

    When POST Login {"email":"freelancerA@test.com","password":"pass123"} → freelancerAToken
    Then status == 200
    When POST SubmitProposal {"id": gig.gig.id, "bid_amount": 4500} → proposal
    Then status == 200

    When POST Login {"email":"client@test.com","password":"pass123"} → clientToken2
    Then status == 200
    When POST AcceptProposal {"id": proposal.proposal.id}
    Then status == 200

    When POST Login {"email":"freelancerA@test.com","password":"pass123"} → freelancerAToken2
    Then status == 200
    When POST SubmitWork {"id": gig.gig.id}
    Then status == 200
    Then response.gig.status == "under_review"

    When POST Login {"email":"client@test.com","password":"pass123"} → clientToken3
    Then status == 200
    When POST ApproveWork {"id": gig.gig.id}
    Then status == 200
    Then response.gig.status == "completed"

  @invariant
  Scenario: Unauthorized Access - Freelancer B cannot submit work on Freelancer A gig
    When POST Register {"email":"clientI@test.com","password":"pass123","role":"client","name":"Client I"}
    Then status == 200
    When POST Register {"email":"freelancerI1@test.com","password":"pass123","role":"freelancer","name":"Freelancer I1"}
    Then status == 200
    When POST Register {"email":"freelancerI2@test.com","password":"pass123","role":"freelancer","name":"Freelancer I2"}
    Then status == 200

    When POST Login {"email":"clientI@test.com","password":"pass123"} → ciToken
    Then status == 200
    When POST CreateGig {"title":"Invariant Gig","description":"Test gig","budget":3000} → iGig
    Then status == 200
    When PUT PublishGig {"id": iGig.gig.id}
    Then status == 200

    When POST Login {"email":"freelancerI1@test.com","password":"pass123"} → fi1Token
    Then status == 200
    When POST SubmitProposal {"id": iGig.gig.id, "bid_amount": 2500} → iProp
    Then status == 200

    When POST Login {"email":"clientI@test.com","password":"pass123"} → ciToken2
    Then status == 200
    When POST AcceptProposal {"id": iProp.proposal.id}
    Then status == 200

    When POST Login {"email":"freelancerI2@test.com","password":"pass123"} → fi2Token
    Then status == 200
    When POST SubmitWork {"id": iGig.gig.id}
    Then status == 403

  @invariant
  Scenario: Invalid State - Cannot approve work when gig is in open state
    When POST Register {"email":"clientS@test.com","password":"pass123","role":"client","name":"Client S"}
    Then status == 200

    When POST Login {"email":"clientS@test.com","password":"pass123"} → csToken
    Then status == 200
    When POST CreateGig {"title":"State Gig","description":"Test state","budget":2000} → sGig
    Then status == 200
    When PUT PublishGig {"id": sGig.gig.id}
    Then status == 200
    When POST ApproveWork {"id": sGig.gig.id}
    Then status == 409
