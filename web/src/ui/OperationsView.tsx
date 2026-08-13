import { useState } from "react";
import { connectionLabel, type ConnectionState } from "../state/connection";
import type { ProjectionViewState } from "../state/projectionStore";
import type { OpsSnapshot, SessionView } from "../api/types";

export interface OperationsViewProps {
  connection: ConnectionState;
  projection: ProjectionViewState;
  errorMessage: string | null;
  tenantId: string;
  session?: SessionView | null;
  unauthenticated?: boolean;
  onSignIn?: () => void;
  onLogout?: () => void;
  ops?: OpsSnapshot | null;
  opsError?: string | null;
  onRelease?: (eventId: string) => void;
  onReplay?: (eventId?: string) => void;
  onRebuild?: () => void;
  controlBusy?: boolean;
  controlError?: string | null;
  controlStatus?: string | null;
}

export function OperationsView(props: OperationsViewProps) {
  const {
    connection,
    projection,
    errorMessage,
    tenantId,
    session,
    unauthenticated,
    onSignIn,
    onLogout,
    ops,
    opsError,
    onRelease,
    onReplay,
    onRebuild,
    controlBusy,
    controlError,
    controlStatus,
  } = props;
  const live = !unauthenticated && connection === "live";
  const showOps = ops !== undefined || opsError !== undefined;
  const showControls = Boolean(onRelease || onReplay || onRebuild);
  const [eventId, setEventId] = useState("");

  return (
    <main className="ops">
      <header className="ops__header">
        <p className="ops__brand">SeshatOps</p>
        <h1 className="ops__title">Northstar inventory projection</h1>
        <p className="ops__subtitle">
          Read-only view of committed Go projection state (Event Spine).
        </p>
      </header>

      {unauthenticated ? (
        <section className="ops__session" data-testid="sign-in-panel">
          <p>
            Sign in through the identity provider. A session is identity only;
            it does not grant tenant or action authorization.
          </p>
          <button type="button" data-testid="sign-in" onClick={onSignIn}>
            Sign in
          </button>
        </section>
      ) : session ? (
        <section className="ops__session" data-testid="session-panel">
          <dl>
            <div>
              <dt>Principal</dt>
              <dd data-testid="session-principal">{session.principal_id}</dd>
            </div>
            <div>
              <dt>Session expires</dt>
              <dd data-testid="session-expires">{session.expires_at}</dd>
            </div>
          </dl>
          <button type="button" data-testid="sign-out" onClick={onLogout}>
            Sign out
          </button>
        </section>
      ) : null}

      <section
        className={`ops__status ops__status--${unauthenticated ? "error" : connection}`}
        aria-live="polite"
        data-testid="connection-status"
        data-connection={unauthenticated ? "error" : connection}
        data-authoritative-live={live ? "true" : "false"}
      >
        <strong>Connection:</strong>{" "}
        {unauthenticated ? "signed out" : connectionLabel(connection)}
        {!live && !unauthenticated && connection !== "loading" ? (
          <span className="ops__status-note">
            {" "}
            Displayed quantities are not guaranteed current.
          </span>
        ) : null}
      </section>

      {errorMessage && !unauthenticated ? (
        <section className="ops__error" role="alert" data-testid="error-banner">
          API error: {errorMessage}
        </section>
      ) : null}

      {unauthenticated ? null : (
        <>
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

          {showOps ? (
            <section className="ops__visibility" data-testid="ops-visibility">
              <h2>Processing visibility</h2>
              <p className="ops__visibility-note">
                Lag, poison/quarantine, and projection freshness for this
                tenant. Go authorizes; this screen does not.
              </p>
              {opsError ? (
                <p data-testid="ops-error" role="alert">
                  Ops API error: {opsError}
                </p>
              ) : null}
              {ops ? (
                <dl>
                  <div>
                    <dt>Projection items</dt>
                    <dd data-testid="ops-item-count">{ops.projection.item_count}</dd>
                  </div>
                  <div>
                    <dt>Projection checksum</dt>
                    <dd data-testid="ops-checksum">{ops.projection.checksum}</dd>
                  </div>
                  <div>
                    <dt>Ops observed at</dt>
                    <dd data-testid="ops-observed-at">{ops.observed_at}</dd>
                  </div>
                  <div>
                    <dt>Outbox pending</dt>
                    <dd data-testid="ops-pending">{ops.backlog.pending}</dd>
                  </div>
                  <div>
                    <dt>Outbox quarantined</dt>
                    <dd data-testid="ops-outbox-quarantined">
                      {ops.backlog.quarantined}
                    </dd>
                  </div>
                  <div>
                    <dt>Oldest unpublished</dt>
                    <dd data-testid="ops-oldest-unpublished">
                      {ops.backlog.oldest_unpublished || "—"}
                    </dd>
                  </div>
                  <div>
                    <dt>Processing applied</dt>
                    <dd data-testid="ops-applied">{ops.processing.applied}</dd>
                  </div>
                  <div>
                    <dt>Quarantined gaps</dt>
                    <dd data-testid="ops-gaps">
                      {ops.processing.quarantined_gap}
                    </dd>
                  </div>
                  <div>
                    <dt>Failures quarantined</dt>
                    <dd data-testid="ops-failures-quarantined">
                      {ops.processing.failures_quarantined}
                    </dd>
                  </div>
                  <div>
                    <dt>Oldest gap</dt>
                    <dd data-testid="ops-oldest-gap">
                      {ops.processing.oldest_gap || "—"}
                    </dd>
                  </div>
                  <div>
                    <dt>Oldest failure</dt>
                    <dd data-testid="ops-oldest-failure">
                      {ops.processing.oldest_failure || "—"}
                    </dd>
                  </div>
                </dl>
              ) : opsError ? null : (
                <p data-testid="ops-loading">Loading processing signals.</p>
              )}
            </section>
          ) : null}

          {showControls ? (
            <section className="ops__controls" data-testid="ops-controls">
              <h2>Privileged controls</h2>
              <p className="ops__visibility-note">
                Go authorizes quarantine release, replay, and rebuild. Hidden
                or disabled buttons are not a security boundary.
              </p>
              <label className="ops__control-field">
                Event ID
                <input
                  data-testid="ops-event-id"
                  value={eventId}
                  onChange={(e) => setEventId(e.target.value)}
                  spellCheck={false}
                />
              </label>
              <div className="ops__control-actions">
                {onRelease ? (
                  <button
                    type="button"
                    data-testid="ops-release"
                    disabled={controlBusy || eventId === ""}
                    onClick={() => onRelease(eventId)}
                  >
                    Release quarantine
                  </button>
                ) : null}
                {onReplay ? (
                  <button
                    type="button"
                    data-testid="ops-replay"
                    disabled={controlBusy}
                    onClick={() => onReplay(eventId || undefined)}
                  >
                    Replay
                  </button>
                ) : null}
                {onRebuild ? (
                  <button
                    type="button"
                    data-testid="ops-rebuild"
                    disabled={controlBusy}
                    onClick={() => onRebuild()}
                  >
                    Rebuild
                  </button>
                ) : null}
              </div>
              {controlError ? (
                <p data-testid="ops-control-error" role="alert">
                  Control API error: {controlError}
                </p>
              ) : null}
              {controlStatus ? (
                <p data-testid="ops-control-status">
                  Last control status: {controlStatus}
                </p>
              ) : null}
            </section>
          ) : null}
        </>
      )}
    </main>
  );
}
