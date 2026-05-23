import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/')({
  component: Index,
})

function Index() {
  return (
    <main className="p-8">
      <h1 className="text-3xl font-bold">Welcome</h1>
    </main>
  )
}
