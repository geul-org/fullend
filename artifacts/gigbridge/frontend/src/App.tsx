import { Routes, Route } from 'react-router-dom'
import Gigs from './pages/gigs'
import GigDetail from './pages/gig-detail'

export default function App() {
  return (
    <Routes>
      <Route path="/gigs" element={<Gigs />} />
      <Route path="/gigs/:id" element={<GigDetail />} />
    </Routes>
  )
}
