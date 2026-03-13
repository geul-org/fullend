# proposal

```mermaid
stateDiagram-v2
    [*] --> pending
    pending --> accepted: AcceptProposal
    pending --> rejected: RejectProposal
```
