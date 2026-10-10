package admin

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Public redirects must complete even if their secondary analytics write fails.
func recordPublicEventAsync(db *pgxpool.Pool, event string, cardID, userID any, metadata string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = db.Exec(ctx, `INSERT INTO analytics_events(event_type,card_id,user_id,metadata) VALUES($1,$2,$3,$4::jsonb)`, event, cardID, userID, metadata)
	}()
}
