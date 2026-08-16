package event

import "fmt"

// Event Spine family constants from docs/design/specifications/event-spine.md.
const (
	EventTypeQuantityDecremented     = "inventory.quantity_decremented"
	EventTypeSupplierRegistered      = "supplier.registered"
	EventTypeIngredientLotReceived   = "ingredient_lot.received"
	EventTypeProductionBatchProduced = "production_batch.produced"
	EventTypeShipmentDispatched      = "shipment.dispatched"

	AggregateTypeInventoryItem   = "inventory_item"
	AggregateTypeSupplier        = "supplier"
	AggregateTypeIngredientLot   = "ingredient_lot"
	AggregateTypeProductionBatch = "production_batch"
	AggregateTypeShipment        = "shipment"

	ProducerSyntheticERP = "synthetic-erp"
	SchemaVersionV1      = int64(1)
)

// Payload is a sealed Event Spine payload. Only types in this package implement it.
type Payload interface {
	isPayload()
	payloadMap() map[string]any
	validateAgainst(env Envelope) error
}

// QuantityDecremented is the inventory.quantity_decremented v1 payload.
type QuantityDecremented struct {
	OrderID             string
	ItemID              string
	QuantityDecremented int64
	QuantityBefore      int64
	QuantityAfter       int64
}

func (QuantityDecremented) isPayload() {}

func (p QuantityDecremented) payloadMap() map[string]any {
	return map[string]any{
		"order_id":             p.OrderID,
		"item_id":              p.ItemID,
		"quantity_decremented": p.QuantityDecremented,
		"quantity_before":      p.QuantityBefore,
		"quantity_after":       p.QuantityAfter,
	}
}

func (p QuantityDecremented) validateAgainst(env Envelope) error {
	if env.EventType != EventTypeQuantityDecremented {
		return fmt.Errorf("%w: payload type mismatch", ErrMalformed)
	}
	if err := requireUTF8(p.OrderID, "payload.order_id"); err != nil {
		return err
	}
	if err := requireUTF8(p.ItemID, "payload.item_id"); err != nil {
		return err
	}
	if err := requireUUID(p.OrderID, "payload.order_id"); err != nil {
		return err
	}
	if err := requireCanonicalID(p.ItemID, "payload.item_id"); err != nil {
		return err
	}
	if p.ItemID != env.AggregateID {
		return fmt.Errorf("%w: payload.item_id must equal aggregate_id", ErrMalformed)
	}
	if p.QuantityDecremented < 1 || p.QuantityDecremented > maxSafeInt {
		return fmt.Errorf("%w: quantity_decremented %d", ErrMalformed, p.QuantityDecremented)
	}
	if p.QuantityBefore < 0 || p.QuantityBefore > maxSafeInt {
		return fmt.Errorf("%w: quantity_before %d", ErrMalformed, p.QuantityBefore)
	}
	if p.QuantityAfter < 0 || p.QuantityAfter > maxSafeInt {
		return fmt.Errorf("%w: quantity_after %d", ErrMalformed, p.QuantityAfter)
	}
	if p.QuantityBefore-p.QuantityDecremented != p.QuantityAfter {
		return fmt.Errorf("%w: quantity arithmetic invariant failed", ErrMalformed)
	}
	return nil
}

// SupplierRegistered is the supplier.registered v1 payload.
type SupplierRegistered struct {
	SupplierID string
}

func (SupplierRegistered) isPayload() {}

func (p SupplierRegistered) payloadMap() map[string]any {
	return map[string]any{"supplier_id": p.SupplierID}
}

func (p SupplierRegistered) validateAgainst(env Envelope) error {
	if env.EventType != EventTypeSupplierRegistered {
		return fmt.Errorf("%w: payload type mismatch", ErrMalformed)
	}
	if err := requireUTF8(p.SupplierID, "payload.supplier_id"); err != nil {
		return err
	}
	if err := requireCanonicalID(p.SupplierID, "payload.supplier_id"); err != nil {
		return err
	}
	if p.SupplierID != env.AggregateID {
		return fmt.Errorf("%w: payload.supplier_id must equal aggregate_id", ErrMalformed)
	}
	return nil
}

// IngredientLotReceived is the ingredient_lot.received v1 payload.
type IngredientLotReceived struct {
	LotID      string
	SupplierID string
	ItemID     string
}

func (IngredientLotReceived) isPayload() {}

func (p IngredientLotReceived) payloadMap() map[string]any {
	return map[string]any{
		"lot_id":      p.LotID,
		"supplier_id": p.SupplierID,
		"item_id":     p.ItemID,
	}
}

func (p IngredientLotReceived) validateAgainst(env Envelope) error {
	if env.EventType != EventTypeIngredientLotReceived {
		return fmt.Errorf("%w: payload type mismatch", ErrMalformed)
	}
	if err := requireUTF8(p.LotID, "payload.lot_id"); err != nil {
		return err
	}
	if err := requireUTF8(p.SupplierID, "payload.supplier_id"); err != nil {
		return err
	}
	if err := requireUTF8(p.ItemID, "payload.item_id"); err != nil {
		return err
	}
	if err := requireCanonicalID(p.LotID, "payload.lot_id"); err != nil {
		return err
	}
	if err := requireCanonicalID(p.SupplierID, "payload.supplier_id"); err != nil {
		return err
	}
	if err := requireCanonicalID(p.ItemID, "payload.item_id"); err != nil {
		return err
	}
	if p.LotID != env.AggregateID {
		return fmt.Errorf("%w: payload.lot_id must equal aggregate_id", ErrMalformed)
	}
	return nil
}

// ProductionBatchProduced is the production_batch.produced v1 payload.
type ProductionBatchProduced struct {
	BatchID string
	LotID   string
}

func (ProductionBatchProduced) isPayload() {}

func (p ProductionBatchProduced) payloadMap() map[string]any {
	return map[string]any{
		"batch_id": p.BatchID,
		"lot_id":   p.LotID,
	}
}

func (p ProductionBatchProduced) validateAgainst(env Envelope) error {
	if env.EventType != EventTypeProductionBatchProduced {
		return fmt.Errorf("%w: payload type mismatch", ErrMalformed)
	}
	if err := requireUTF8(p.BatchID, "payload.batch_id"); err != nil {
		return err
	}
	if err := requireUTF8(p.LotID, "payload.lot_id"); err != nil {
		return err
	}
	if err := requireCanonicalID(p.BatchID, "payload.batch_id"); err != nil {
		return err
	}
	if err := requireCanonicalID(p.LotID, "payload.lot_id"); err != nil {
		return err
	}
	if p.BatchID != env.AggregateID {
		return fmt.Errorf("%w: payload.batch_id must equal aggregate_id", ErrMalformed)
	}
	return nil
}

// ShipmentDispatched is the shipment.dispatched v1 payload.
type ShipmentDispatched struct {
	ShipmentID string
	BatchID    string
	OrderID    string
}

func (ShipmentDispatched) isPayload() {}

func (p ShipmentDispatched) payloadMap() map[string]any {
	return map[string]any{
		"shipment_id": p.ShipmentID,
		"batch_id":    p.BatchID,
		"order_id":    p.OrderID,
	}
}

func (p ShipmentDispatched) validateAgainst(env Envelope) error {
	if env.EventType != EventTypeShipmentDispatched {
		return fmt.Errorf("%w: payload type mismatch", ErrMalformed)
	}
	if err := requireUTF8(p.ShipmentID, "payload.shipment_id"); err != nil {
		return err
	}
	if err := requireUTF8(p.BatchID, "payload.batch_id"); err != nil {
		return err
	}
	if err := requireUTF8(p.OrderID, "payload.order_id"); err != nil {
		return err
	}
	if err := requireCanonicalID(p.ShipmentID, "payload.shipment_id"); err != nil {
		return err
	}
	if err := requireCanonicalID(p.BatchID, "payload.batch_id"); err != nil {
		return err
	}
	if err := requireUUID(p.OrderID, "payload.order_id"); err != nil {
		return err
	}
	if p.ShipmentID != env.AggregateID {
		return fmt.Errorf("%w: payload.shipment_id must equal aggregate_id", ErrMalformed)
	}
	return nil
}

// AsQuantityDecremented returns the inventory payload when env is that family.
func AsQuantityDecremented(env Envelope) (QuantityDecremented, bool) {
	p, ok := env.Payload.(QuantityDecremented)
	return p, ok
}

// WithQuantityDecremented returns a copy of env after applying fn to the inventory payload.
func WithQuantityDecremented(env Envelope, fn func(*QuantityDecremented)) Envelope {
	p, ok := AsQuantityDecremented(env)
	if !ok {
		return env
	}
	fn(&p)
	env.Payload = p
	return env
}

func familyAggregateType(eventType string, schema int64) (string, error) {
	var agg string
	switch eventType {
	case EventTypeQuantityDecremented:
		agg = AggregateTypeInventoryItem
	case EventTypeSupplierRegistered:
		agg = AggregateTypeSupplier
	case EventTypeIngredientLotReceived:
		agg = AggregateTypeIngredientLot
	case EventTypeProductionBatchProduced:
		agg = AggregateTypeProductionBatch
	case EventTypeShipmentDispatched:
		agg = AggregateTypeShipment
	default:
		return "", fmt.Errorf("%w: event_type %q", ErrUnsupported, eventType)
	}
	if schema != SchemaVersionV1 {
		return "", fmt.Errorf("%w: event_schema_version %d", ErrUnsupported, schema)
	}
	return agg, nil
}

func payloadFromObject(eventType string, schema int64, obj map[string]any) (Payload, error) {
	if _, err := familyAggregateType(eventType, schema); err != nil {
		return nil, err
	}
	switch eventType {
	case EventTypeQuantityDecremented:
		return quantityDecrementedFromObject(obj)
	case EventTypeSupplierRegistered:
		return supplierRegisteredFromObject(obj)
	case EventTypeIngredientLotReceived:
		return ingredientLotReceivedFromObject(obj)
	case EventTypeProductionBatchProduced:
		return productionBatchProducedFromObject(obj)
	case EventTypeShipmentDispatched:
		return shipmentDispatchedFromObject(obj)
	default:
		return nil, fmt.Errorf("%w: event_type %q", ErrUnsupported, eventType)
	}
}

func quantityDecrementedFromObject(obj map[string]any) (QuantityDecremented, error) {
	if err := requirePayloadKeys(obj, "order_id", "item_id", "quantity_decremented", "quantity_before", "quantity_after"); err != nil {
		return QuantityDecremented{}, err
	}
	var p QuantityDecremented
	var err error
	if p.OrderID, err = asString(obj["order_id"], "payload.order_id"); err != nil {
		return QuantityDecremented{}, err
	}
	if p.ItemID, err = asString(obj["item_id"], "payload.item_id"); err != nil {
		return QuantityDecremented{}, err
	}
	if p.QuantityDecremented, err = asInt(obj["quantity_decremented"], "payload.quantity_decremented"); err != nil {
		return QuantityDecremented{}, err
	}
	if p.QuantityBefore, err = asInt(obj["quantity_before"], "payload.quantity_before"); err != nil {
		return QuantityDecremented{}, err
	}
	if p.QuantityAfter, err = asInt(obj["quantity_after"], "payload.quantity_after"); err != nil {
		return QuantityDecremented{}, err
	}
	return p, nil
}

func supplierRegisteredFromObject(obj map[string]any) (SupplierRegistered, error) {
	if err := requirePayloadKeys(obj, "supplier_id"); err != nil {
		return SupplierRegistered{}, err
	}
	id, err := asString(obj["supplier_id"], "payload.supplier_id")
	if err != nil {
		return SupplierRegistered{}, err
	}
	return SupplierRegistered{SupplierID: id}, nil
}

func ingredientLotReceivedFromObject(obj map[string]any) (IngredientLotReceived, error) {
	if err := requirePayloadKeys(obj, "lot_id", "supplier_id", "item_id"); err != nil {
		return IngredientLotReceived{}, err
	}
	var p IngredientLotReceived
	var err error
	if p.LotID, err = asString(obj["lot_id"], "payload.lot_id"); err != nil {
		return IngredientLotReceived{}, err
	}
	if p.SupplierID, err = asString(obj["supplier_id"], "payload.supplier_id"); err != nil {
		return IngredientLotReceived{}, err
	}
	if p.ItemID, err = asString(obj["item_id"], "payload.item_id"); err != nil {
		return IngredientLotReceived{}, err
	}
	return p, nil
}

func productionBatchProducedFromObject(obj map[string]any) (ProductionBatchProduced, error) {
	if err := requirePayloadKeys(obj, "batch_id", "lot_id"); err != nil {
		return ProductionBatchProduced{}, err
	}
	var p ProductionBatchProduced
	var err error
	if p.BatchID, err = asString(obj["batch_id"], "payload.batch_id"); err != nil {
		return ProductionBatchProduced{}, err
	}
	if p.LotID, err = asString(obj["lot_id"], "payload.lot_id"); err != nil {
		return ProductionBatchProduced{}, err
	}
	return p, nil
}

func shipmentDispatchedFromObject(obj map[string]any) (ShipmentDispatched, error) {
	if err := requirePayloadKeys(obj, "shipment_id", "batch_id", "order_id"); err != nil {
		return ShipmentDispatched{}, err
	}
	var p ShipmentDispatched
	var err error
	if p.ShipmentID, err = asString(obj["shipment_id"], "payload.shipment_id"); err != nil {
		return ShipmentDispatched{}, err
	}
	if p.BatchID, err = asString(obj["batch_id"], "payload.batch_id"); err != nil {
		return ShipmentDispatched{}, err
	}
	if p.OrderID, err = asString(obj["order_id"], "payload.order_id"); err != nil {
		return ShipmentDispatched{}, err
	}
	return p, nil
}

func requirePayloadKeys(obj map[string]any, keys ...string) error {
	allowed := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		allowed[k] = struct{}{}
		if _, ok := obj[k]; !ok {
			return fmt.Errorf("%w: missing payload.%s", ErrMalformed, k)
		}
	}
	for k := range obj {
		if _, ok := allowed[k]; !ok {
			return fmt.Errorf("%w: unknown payload field %q", ErrMalformed, k)
		}
	}
	return nil
}
