import './App.css'
import Dashboard from "./pages/Dashboard.tsx";
import {BrowserRouter, Route, Routes} from "react-router";
import {Layout} from "./component/Layout.tsx";

function App() {
  return (
    <>
      <BrowserRouter>
        <Routes>
          <Route element={<Layout/>}>
            <Route path={"/"} element={<Dashboard/>}/>
          </Route>
        </Routes>
      </BrowserRouter>
      {/*<main className="h-screen w-screen p-0">*/}
      {/*  <PanelGroup direction="horizontal" className="h-full border overflow-hidden">*/}
      {/*    <Panel defaultSize={30} minSize={20} maxSize={50} className="p-4 bg-gray-100">*/}
      {/*      <Sidebar/>*/}
      {/*    </Panel>*/}

      {/*    <PanelResizeHandle className="w-2 bg-gray-300 hover:bg-gray-400 cursor-col-resize"/>*/}

      {/*    <Panel className="p-4 bg-white">*/}
      {/*      <Dashboard/>*/}
      {/*    </Panel>*/}
      {/*  </PanelGroup>*/}
      {/*</main>*/}
    </>
  )
}

export default App
