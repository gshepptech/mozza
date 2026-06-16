import React, { Suspense } from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { AuthProvider, useAuth } from "./context/AuthContext";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Toaster } from "@/components/ui/sonner";

// Lazy-load all page components for route-based code splitting
const LandingPage = React.lazy(() => import("./pages/LandingPage"));
const LoginPage = React.lazy(() => import("./pages/LoginPage"));
const RegisterPage = React.lazy(() => import("./pages/RegisterPage"));
const DashboardPage = React.lazy(() => import("./pages/DashboardPage"));
const OverviewPage = React.lazy(() => import("./pages/OverviewPage"));
const ApplicationsPage = React.lazy(() => import("./pages/ApplicationsPage"));
const ApplicationDetailPage = React.lazy(() => import("./pages/ApplicationDetailPage"));
const RecipeListPage = React.lazy(() => import("./pages/RecipeListPage"));
const RecipeBuilderPage = React.lazy(() => import("./pages/RecipeBuilderPage"));
const DeploymentsPage = React.lazy(() => import("./pages/DeploymentsPage"));
const EnvironmentsPage = React.lazy(() => import("./pages/EnvironmentsPage"));
const MonitoringPage = React.lazy(() => import("./pages/MonitoringPage"));
const StatusPage = React.lazy(() => import("./pages/StatusPage"));
const DoctorPage = React.lazy(() => import("./pages/DoctorPage"));
const TeamSettingsPage = React.lazy(() => import("./pages/TeamSettingsPage"));
const CreateTeamPage = React.lazy(() => import("./pages/CreateTeamPage"));
const DeployWizardPage = React.lazy(() => import("./pages/DeployWizardPage"));
const ClustersPage = React.lazy(() => import("./pages/ClustersPage"));
const ProfilePage = React.lazy(() => import("./pages/ProfilePage"));
const MarketplacePage = React.lazy(() => import("./pages/MarketplacePage"));

function PageLoader() {
  return (
    <div style={{ display: "flex", justifyContent: "center", alignItems: "center", minHeight: "200px" }}>
      <div style={{
        width: 32,
        height: 32,
        border: "3px solid #333",
        borderTopColor: "#ff6b35",
        borderRadius: "50%",
        animation: "spin 0.8s linear infinite",
      }} />
      <style>{`@keyframes spin { to { transform: rotate(360deg); } }`}</style>
    </div>
  );
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <div className="flex flex-col items-center gap-3">
          <div className="h-8 w-8 border-2 border-brand border-t-transparent rounded-full animate-spin" />
          <span className="text-sm text-muted-foreground">Loading...</span>
        </div>
      </div>
    );
  }

  if (!user) {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <TooltipProvider>
          <Suspense fallback={<PageLoader />}>
            <Routes>
              {/* Public routes */}
              <Route path="/" element={<LandingPage />} />
              <Route path="/login" element={<LoginPage />} />
              <Route path="/register" element={<RegisterPage />} />

              {/* Protected app routes */}
              <Route
                path="/app"
                element={
                  <ProtectedRoute>
                    <DashboardPage />
                  </ProtectedRoute>
                }
              >
                <Route index element={<OverviewPage />} />
                <Route path="applications" element={<ApplicationsPage />} />
                <Route path="applications/:id" element={<ApplicationDetailPage />} />
                <Route path="recipes" element={<RecipeListPage />} />
                <Route path="marketplace" element={<MarketplacePage />} />
                <Route path="recipes/:id" element={<RecipeBuilderPage />} />
                <Route path="deploy" element={<DeployWizardPage />} />
                <Route path="deployments" element={<DeploymentsPage />} />
                <Route path="environments" element={<EnvironmentsPage />} />
                <Route path="monitoring" element={<MonitoringPage />} />
                <Route path="status" element={<StatusPage />} />
                <Route path="clusters" element={<ClustersPage />} />
                <Route path="doctor" element={<DoctorPage />} />
                <Route path="teams/new" element={<CreateTeamPage />} />
                <Route path="teams/:id" element={<TeamSettingsPage />} />
                <Route path="profile" element={<ProfilePage />} />
              </Route>

              {/* Catch-all redirect */}
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
          </Suspense>
          <Toaster />
        </TooltipProvider>
      </AuthProvider>
    </BrowserRouter>
  );
}
