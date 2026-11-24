// internal/repository/replica_repository.go
package repository

import (
	"database/sql"

	"github.com/mbagusaditya/scrobbles-cli/internal/repository/replica"
)

type ReplicaRepository = replica.Repository

func NewReplicaRepository(db *sql.DB) *ReplicaRepository {
	return replica.NewRepository(db)
}
