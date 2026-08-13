import { OperationsView } from "./ui/OperationsView";
import { useInventoryProjection } from "./state/useInventoryProjection";
import { useOpsVisibility } from "./state/useOpsVisibility";
import { useSession } from "./state/useSession";
import { emptyProjectionView } from "./state/projectionStore";
import { loginUrl } from "./api/session";
import { UNAUTHENTICATED, type SessionView } from "./api/types";
import { NORTHSTAR_TENANT_ID } from "./fixtures/northstar";

// Empty default uses the Vite /v1 and /auth proxy (same origin). Set an
// absolute URL only when the Go API advertises CORS for that origin.
const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? "").replace(
  /\/$/,
  "",
);

const TENANT_ID =
  import.meta.env.VITE_TENANT_ID || NORTHSTAR_TENANT_ID;

function goToLogin() {
  window.location.assign(loginUrl(API_BASE_URL));
}

export function App() {
  const { status, session, errorMessage, logout } = useSession({
    baseUrl: API_BASE_URL,
  });

  if (status === "error") {
    return (
      <OperationsView
        connection="error"
        projection={emptyProjectionView()}
        errorMessage={errorMessage}
        tenantId={TENANT_ID}
      />
    );
  }

  if (status !== "authenticated" || session === null) {
    return (
      <OperationsView
        connection="loading"
        projection={emptyProjectionView()}
        errorMessage={status === "loading" ? null : errorMessage}
        tenantId={TENANT_ID}
        unauthenticated={status === "unauthenticated"}
        onSignIn={goToLogin}
      />
    );
  }

  return (
    <AuthenticatedView session={session} onLogout={() => void logout()} />
  );
}

function AuthenticatedView(props: {
  session: SessionView;
  onLogout: () => void;
}) {
  const { connection, projection, errorMessage } = useInventoryProjection({
    baseUrl: API_BASE_URL,
    tenantId: TENANT_ID,
  });
  const ops = useOpsVisibility({
    baseUrl: API_BASE_URL,
    tenantId: TENANT_ID,
  });

  if (errorMessage === UNAUTHENTICATED) {
    return (
      <OperationsView
        connection="error"
        projection={emptyProjectionView()}
        errorMessage={null}
        tenantId={TENANT_ID}
        unauthenticated
        onSignIn={goToLogin}
      />
    );
  }

  return (
    <OperationsView
      connection={connection}
      projection={projection}
      errorMessage={errorMessage}
      tenantId={TENANT_ID}
      session={props.session}
      onLogout={props.onLogout}
      ops={ops.snapshot}
      opsError={ops.errorMessage}
    />
  );
}
