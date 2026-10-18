package admin

import "context"

type operatorKey struct{}

func withOperator(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, operatorKey{}, name)
}

func operatorFrom(ctx context.Context) string {
	name, _ := ctx.Value(operatorKey{}).(string)
	return name
}
