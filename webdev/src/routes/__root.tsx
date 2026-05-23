import { createRootRoute, Link, Outlet } from '@tanstack/react-router'
import { TanStackRouterDevtools } from '@tanstack/router-devtools'

export const Route = createRootRoute({
  component: () => (
    <>
      <nav className="flex gap-4 p-4 border-b">
        <Link to="/" className="hover:underline [&.active]:font-bold">Home</Link>
      </nav>
      <Outlet />
      <TanStackRouterDevtools />
    </>
  ),
})
