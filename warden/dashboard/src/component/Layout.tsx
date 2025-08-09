import {Panel, PanelGroup, PanelResizeHandle} from "react-resizable-panels";
import Sidebar from "./Sidebar.tsx";
import {Outlet} from "react-router";

const Layout = () => {
  return <>
    <main className="h-screen w-screen p-0">
      <PanelGroup direction="horizontal" className="h-full border overflow-hidden">
        <Panel defaultSize={30} minSize={20} maxSize={50} className="p-4 bg-gray-100">
          <Sidebar/>
        </Panel>

        <PanelResizeHandle className="w-2 bg-gray-300 hover:bg-gray-400 cursor-col-resize"/>

        <Panel className="p-4 bg-white">
          {/*<Dashboard/>*/}
          <Outlet/>
        </Panel>
      </PanelGroup>
    </main>
  </>
}

export {Layout}