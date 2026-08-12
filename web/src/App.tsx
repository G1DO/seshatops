import { OperationsView } from "./ui/OperationsView";
import { useInventoryProjection } from "./state/useInventoryProjection";
import { NORTHSTAR_TENANT_ID } from "./fixtures/northstar";

// Empty default uses the Vite /v1 proxy (same origin). Set an absolute URL only
// when the Go API advertises CORS for that origin.
const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? "").replace(
  /\/$/,
  "",
);

const TENANT_ID =
  import.meta.env.VITE_TENANT_ID || NORTHSTAR_TENANT_ID;

export function App() {
  const { connection, projection, errorMessage } = useInventoryProjection({
    baseUrl: API_BASE_URL,
    tenantId: TENANT_ID,
  });

  return (
    <OperationsView
      connection={connection}
      projection={projection}
      errorMessage={errorMessage}
      tenantId={TENANT_ID}
    />
  );
}
