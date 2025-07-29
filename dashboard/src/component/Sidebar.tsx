import {NavLink} from "react-router";
import {Link} from "@heroui/react";

export default function Sidebar() {
  const linkClass =
    "block px-4 py-2 text-sm hover:bg-gray-200 rounded transition";

  return (
    <aside className="w-64 bg-gray-100 p-4  h-full">
      <h2 className="text-xl font-bold mb-6">导航</h2>
      <nav className="space-y-2">
        <NavLink to="/" end className={({ isActive }) => isActive ? `${linkClass} bg-gray-300` : linkClass}>
          <Link>
            Dashboard
          </Link>
        </NavLink>
        <NavLink to="/controller" className={({ isActive }) => isActive ? `${linkClass} bg-gray-300` : linkClass}>
          Controller 状态
        </NavLink>
        <NavLink to="/agent" className={({ isActive }) => isActive ? `${linkClass} bg-gray-300` : linkClass}>
          Agent 状态
        </NavLink>
      </nav>
    </aside>
  );
}