import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useParams } from 'react-router-dom'
import { useForm } from 'react-hook-form'
import { api } from '../api'

export default function GigDetail() {
  const { id } = useParams()
  const queryClient = useQueryClient()

  const { data: getGigData, isLoading: getGigDataLoading, error: getGigDataError } = useQuery({
    queryKey: ['GetGig', id],
    queryFn: () => api.GetGig({ id: id }),
  })

  const publishGigMutation = useMutation({
    mutationFn: (data: any) => api.PublishGig({ ...data, id: id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['GetGig'] })
    },
  })

  const submitWorkMutation = useMutation({
    mutationFn: (data: any) => api.SubmitWork({ ...data, id: id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['GetGig'] })
    },
  })

  const approveWorkMutation = useMutation({
    mutationFn: (data: any) => api.ApproveWork({ ...data, id: id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['GetGig'] })
    },
  })

  const submitProposalForm = useForm()
  const submitProposalMutation = useMutation({
    mutationFn: (data: any) => api.SubmitProposal({ ...data, id: id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['GetGig'] })
    },
  })

  return (
    <div>
      <title>GigBridge - Gig Detail</title>
      <h1>Gig Detail</h1>
      {getGigDataLoading && <div>로딩 중...</div>}
      {getGigDataError && <div>오류가 발생했습니다</div>}
      {getGigData && (
        <section>
          <h2>{getGigData.gig.title}</h2>
          <p>{getGigData.gig.description}</p>
          <span>{getGigData.gig.budget}</span>
          <span>{getGigData.gig.status}</span>
          <button onClick={() => publishGigMutation.mutate({})}>Publish</button>
          <button onClick={() => submitWorkMutation.mutate({})}>Submit Work</button>
          <button onClick={() => approveWorkMutation.mutate({})}>Approve Work</button>
          <section>
            <h3>Submit Proposal</h3>
            <form onSubmit={submitProposalForm.handleSubmit((data) => submitProposalMutation.mutate(data))}>
              <input type="number" placeholder="Bid Amount" {...submitProposalForm.register('bid_amount', { valueAsNumber: true })} />
              <button type="submit">Submit Proposal</button>
            </form>
          </section>
        </section>
      )}
    </div>
  )
}
