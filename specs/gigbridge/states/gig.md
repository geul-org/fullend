# gig

```mermaid
stateDiagram-v2
    [*] --> draft
    draft --> open: PublishGig
    open --> in_progress: AcceptProposal
    in_progress --> under_review: SubmitWork
    under_review --> completed: ApproveWork
    under_review --> disputed: RaiseDispute
```
