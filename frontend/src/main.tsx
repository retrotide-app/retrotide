import "./api/globalErrorHandlers";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClientProvider } from "@tanstack/react-query";
import App from "./App";
import { queryClient } from "./api/queryClient";
import { reportClientError } from "./api/telemetry";
import { RootErrorBoundary } from "./components/RootErrorBoundary/RootErrorBoundary";
import { AuthProvider } from "./context/AuthContext";
import { OrgProvider } from "./context/OrgContext";
import { SiteInfoProvider } from "./context/SiteInfoContext";
import { ThemeProvider } from "./context/ThemeContext";
import { setPlatformErrorReporter } from "./platform/errorReporter";
import "./styles/variables.css";
import "./styles/global.css";

setPlatformErrorReporter(reportClientError);

const container = document.getElementById("root");

if (container === null) {
    throw new Error("index.html has no #root element");
}

createRoot(container, {
    onUncaughtError: (error, errorInfo) => {
        reportClientError(error, { source: "uncaught", componentStack: errorInfo.componentStack });
    },
    onCaughtError: (error, errorInfo) => {
        reportClientError(error, { source: "caught", componentStack: errorInfo.componentStack });
    },
    onRecoverableError: (error, errorInfo) => {
        reportClientError(error, { source: "recoverable", componentStack: errorInfo.componentStack });
    },
}).render(
    <StrictMode>
        <RootErrorBoundary>
            <QueryClientProvider client={queryClient}>
                <SiteInfoProvider>
                    <AuthProvider>
                        <OrgProvider>
                            <ThemeProvider>
                                <App />
                            </ThemeProvider>
                        </OrgProvider>
                    </AuthProvider>
                </SiteInfoProvider>
            </QueryClientProvider>
        </RootErrorBoundary>
    </StrictMode>,
);
