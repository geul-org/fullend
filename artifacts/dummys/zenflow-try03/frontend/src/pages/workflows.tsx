// fullend:gen ssot=frontend/workflows.html contract=0945489
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { api } from '../api'

export default function Workflows() {
  const queryClient = useQueryClient()

  const [page, setPage] = useState(1)
  const [limit] = useState(20)
  const [sortBy, setSortBy] = useState('created_at')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc')
  const [filters, setFilters] = useState<Record<string, string>>({})

  const { data: listWorkflowsData, isLoading: listWorkflowsDataLoading, error: listWorkflowsDataError } = useQuery({
    queryKey: ['ListWorkflows', page, limit, sortBy, sortDir, filters],
    queryFn: () => api.ListWorkflows({ page, limit, sortBy, sortDir, ...filters }),
  })

  const createWorkflowForm = useForm()
  const createWorkflowMutation = useMutation({
    mutationFn: (data: any) => api.CreateWorkflow(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['ListWorkflows'] })
    },
  })

  return (
    <div>
      <title>Workflows</title>
      <h1>Workflows</h1>
      {listWorkflowsDataLoading && <div>로딩 중...</div>}
      {listWorkflowsDataError && <div>오류가 발생했습니다</div>}
      {listWorkflowsData && (
        <section>
          <div className="flex gap-2 mb-4">
            <input placeholder="status" value={filters.status ?? ''} className="px-3 py-2 border rounded" onChange={(e) => setFilters(f => ({ ...f, status: e.target.value }))} />
          </div>
          <div className="flex gap-2 mb-4">
            <button onClick={() => { setSortBy('created_at'); setSortDir(d => d === 'asc' ? 'desc' : 'asc') }}>
              created_at {sortBy === 'created_at' ? (sortDir === 'asc' ? '↑' : '↓') : ''}
            </button>
          </div>
          <div>
            {listWorkflowsData.items?.map((item: any, index: number) => (
              <h3 key={index}>
              </h3>
            ))}
          </div>
          {listWorkflowsData.items?.length === 0 && (
            <div>
              <p>No workflows found.</p>
            </div>
          )}
          <div className="flex justify-between items-center mt-4">
            <button disabled={page <= 1} onClick={() => setPage(p => p - 1)}>이전</button>
            <span>{page} / {Math.ceil((listWorkflowsData?.total ?? 0) / limit)}</span>
            <button disabled={!listWorkflowsData?.total || page * limit >= listWorkflowsData.total} onClick={() => setPage(p => p + 1)}>다음</button>
          </div>
        </section>
      )}
      <form onSubmit={createWorkflowForm.handleSubmit((data) => createWorkflowMutation.mutate(data))}>
        <input type="text" placeholder="Workflow title" {...createWorkflowForm.register('title')} />
        <input type="text" placeholder="Trigger event" {...createWorkflowForm.register('trigger_event')} />
        <button type="submit">Create Workflow</button>
      </form>
    </div>
  )
}
