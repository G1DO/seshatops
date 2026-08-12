import { connectionLabel, type ConnectionState } from "../state/connection";
import type { ProjectionViewState } from "../state/projectionStore";

export interface OperationsViewProps {
  connection: ConnectionState;
  projection: ProjectionViewState;
  errorMessage: string | null;
  tenantId: string;
}

export function OperationsView(props: OperationsViewProps) {
  const { connection, projection, errorMessage, tenantId } = props;
  const live = connection === "live";

  return (
    <main className="ops">
      <header className="ops__header">
        <p className="ops__brand">SeshatOps</p>
        <h1 className="ops__title">Northstar inventory projection</h1>
        <p className="ops__subtitle">
          Read-only view of committed Go projection state (Event Spine).
        </p>
      </header>

      <section
        className={`ops__status ops__status--${connection}`}
        aria-live="polite"
        data-testid="connection-status"
        data-connection={connection}
        data-authoritative-live={live ? "true" : "false"}
      >
        <strong>Connection:</strong> {connectionLabel(connection)}
        {!live && connection !== "loading" ? (
          <span className="ops__status-note">
            {" "}
            Displayed quantities are not guaranteed current.
          </span>
        ) : null}
      </section>

      {errorMessage ? (
        <section className="ops__error" role="alert" data-testid="error-banner">
          API error: {errorMessage}
        </section>
      ) : null}

      <section className="ops__meta" data-testid="freshness-meta">
        <dl>
          <div>
            <dt>Tenant</dt>
            <dd>{tenantId}</dd>
          </div>
          <div>
            <dt>Checksum</dt>
            <dd data-testid="checksum">{projection.checksum || "—"}</dd>
          </div>
          <div>
            <dt>Observed at</dt>
            <dd data-testid="observed-at">{projection.observed_at || "—"}</dd>
          </div>
          <div>
            <dt>Last applied event</dt>
            <dd data-testid="last-applied-event">
              {projection.last_applied_event_id || "—"}
            </dd>
          </div>
        </dl>
      </section>

      <section className="ops__inventory" aria-label="Inventory projection">
        <h2>Inventory</h2>
        {projection.items.length === 0 ? (
          <p data-testid="empty-items">No committed projection rows.</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th scope="col">Item</th>
                <th scope="col">Before</th>
                <th scope="col">After (on hand)</th>
                <th scope="col">Aggregate version</th>
              </tr>
            </thead>
            <tbody>
              {projection.items.map((item) => (
                <tr key={item.item_id} data-testid={`item-${item.item_id}`}>
                  <td>{item.item_id}</td>
                  <td data-testid={`before-${item.item_id}`}>
                    {item.previous_quantity_on_hand === null
                      ? "—"
                      : item.previous_quantity_on_hand}
                  </td>
                  <td data-testid={`after-${item.item_id}`}>
                    {item.quantity_on_hand}
                  </td>
                  <td data-testid={`version-${item.item_id}`}>
                    {item.aggregate_version}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </main>
  );
}
