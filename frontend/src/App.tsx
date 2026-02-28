import { BrowserRouter, Routes, Route } from "react-router-dom";
import { ToastProvider } from "./context/toast-context";
import { AppShell } from "./components/AppShell";
import { Dashboard } from "./pages/Dashboard";
import { Scan } from "./pages/Scan";
import { Settings } from "./pages/Settings";

export default function App() {
  return (
    <ToastProvider>
      <BrowserRouter>
        <Routes>
          <Route element={<AppShell />}>
            <Route index element={<Dashboard />} />
            <Route path="scan" element={<Scan />} />
            <Route path="settings" element={<Settings />} />
            <Route
              path="*"
              element={
                <div className="flex items-center justify-center h-64 text-content-muted text-sm">
                  Page not found
                </div>
              }
            />
          </Route>
        </Routes>
      </BrowserRouter>
    </ToastProvider>
  );
}
