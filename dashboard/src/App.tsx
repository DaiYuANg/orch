import './App.css'
import {Button} from "@heroui/react";
import {Panel, PanelGroup, PanelResizeHandle} from "react-resizable-panels";

function App() {
  return (
    <>
      <main className="h-screen w-screen p-0">
        <PanelGroup direction="horizontal" className="h-full border overflow-hidden">
          <Panel defaultSize={30} minSize={20} maxSize={50} className="p-4 bg-gray-100">
            <div className="h-full">
              <h2 className="text-lg font-bold mb-4">左侧面板</h2>
              <Button>test</Button>
            </div>
          </Panel>

          <PanelResizeHandle className="w-2 bg-gray-300 hover:bg-gray-400 cursor-col-resize"/>

          <Panel className="p-4 bg-white">
            <h2 className="text-lg font-bold mb-4">右侧内容</h2>
            <p>这里是右边区域的内容</p>
          </Panel>
        </PanelGroup>
      </main>
    </>
  )
}

export default App
