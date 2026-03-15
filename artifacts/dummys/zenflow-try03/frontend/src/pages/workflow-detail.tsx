// fullend:gen ssot=frontend/workflow-detail.html contract=431099c
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useParams } from 'react-router-dom'
import { useForm } from 'react-hook-form'
import { api } from '../api'

export default function WorkflowDetail() {
  const { id } = useParams()
  const queryClient = useQueryClient()

  const { data: getWorkflowData, isLoading: getWorkflowDataLoading, error: getWorkflowDataError } = useQuery({
    queryKey: ['GetWorkflow', id],
    queryFn: () => api.GetWorkflow({ id: id }),
  })

  const { data: listActionsData, isLoading: listActionsDataLoading, error: listActionsDataError } = useQuery({
    queryKey: ['ListActions', id],
    queryFn: () => api.ListActions({ id: id }),
  })

  const activateWorkflowMutation = useMutation({
    mutationFn: (data: any) => api.ActivateWorkflow({ ...data, id: id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['GetWorkflow'] })
      queryClient.invalidateQueries({ queryKey: ['ListActions'] })
    },
  })

  const pauseWorkflowMutation = useMutation({
    mutationFn: (data: any) => api.PauseWorkflow({ ...data, id: id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['GetWorkflow'] })
      queryClient.invalidateQueries({ queryKey: ['ListActions'] })
    },
  })

  const archiveWorkflowMutation = useMutation({
    mutationFn: (data: any) => api.ArchiveWorkflow({ ...data, id: id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['GetWorkflow'] })
      queryClient.invalidateQueries({ queryKey: ['ListActions'] })
    },
  })

  const executeWorkflowMutation = useMutation({
    mutationFn: (data: any) => api.ExecuteWorkflow({ ...data, id: id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['GetWorkflow'] })
      queryClient.invalidateQueries({ queryKey: ['ListActions'] })
    },
  })

  const addActionForm = useForm()
  const addActionMutation = useMutation({
    mutationFn: (data: any) => api.AddAction({ ...data, id: id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['GetWorkflow'] })
      queryClient.invalidateQueries({ queryKey: ['ListActions'] })
    },
  })

  return (
    <div>
      <title>Workflow Detail</title>
      {getWorkflowDataLoading && <div>로딩 중...</div>}
      {getWorkflowDataError && <div>오류가 발생했습니다</div>}
      {getWorkflowData && (
        <section>
          <h1>{getWorkflowData.workflow.title}</h1>
          <p>{getWorkflowData.workflow.status}</p>
          <p>{getWorkflowData.workflow.trigger_event}</p>
        </section>
      )}
      {listActionsDataLoading && <div>로딩 중...</div>}
      {listActionsDataError && <div>오류가 발생했습니다</div>}
      {listActionsData && (
        <section>
          <h2>Actions</h2>
          <div>
            {listActionsData.actions?.map((item: any, index: number) => (
              <span key={index}>
              </span>
            ))}
          </div>
        </section>
      )}
      <form><button onClick={() => activateWorkflowMutation.mutate({})}>Activate</button></form>
      <form><button onClick={() => pauseWorkflowMutation.mutate({})}>Pause</button></form>
      <form><button onClick={() => archiveWorkflowMutation.mutate({})}>Archive</button></form>
      <form><button onClick={() => executeWorkflowMutation.mutate({})}>Execute</button></form>
      <form onSubmit={addActionForm.handleSubmit((data) => addActionMutation.mutate(data))}>
        <input type="text" placeholder="Action type" {...addActionForm.register('action_type')} />
        <input type="number" placeholder="Order" {...addActionForm.register('sequence_order', { valueAsNumber: true })} />
        <button type="submit">Add Action</button>
      </form>
    </div>
  )
}
