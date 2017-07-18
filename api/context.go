package api

import "context"

type contextKey string

func (c contextKey) String() string {
	return "api private context key " + string(c)
}

const (
	userIDKey  = contextKey("user_id")
	orderIDKey = contextKey("order_id")
)

func getUserID(ctx context.Context) string {
	id, _ := ctx.Value(userIDKey).(string)
	return id
}
func withUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func getOrderID(ctx context.Context) string {
	id, _ := ctx.Value(orderIDKey).(string)
	return id
}
func withOrderID(ctx context.Context, orderID string) context.Context {
	return context.WithValue(ctx, orderIDKey, orderID)
}
