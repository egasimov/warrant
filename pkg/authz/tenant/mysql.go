package tenant

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/warrant-dev/warrant/pkg/database"
	"github.com/warrant-dev/warrant/pkg/middleware"
	"github.com/warrant-dev/warrant/pkg/service"
)

type MySQLRepository struct {
	database.SQLRepository
}

func NewMySQLRepository(db *database.MySQL) MySQLRepository {
	return MySQLRepository{
		database.NewSQLRepository(&db.SQL),
	}
}

func (repo MySQLRepository) Create(ctx context.Context, model Model) (int64, error) {
	result, err := repo.DB.ExecContext(
		ctx,
		`
			INSERT INTO tenant (
				tenantId,
				objectId,
				name
			) VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE
				objectId = ?,
				name = ?,
				createdAt = CURRENT_TIMESTAMP(6),
				deletedAt = NULL
		`,
		model.GetTenantId(),
		model.GetObjectId(),
		model.GetName(),
		model.GetObjectId(),
		model.GetName(),
	)

	if err != nil {
		return 0, errors.Wrap(err, "Unable to create Tenant")
	}

	newTenantId, err := result.LastInsertId()
	if err != nil {
		return 0, service.NewInternalError("Unable to create Tenant")
	}

	return newTenantId, nil
}

func (repo MySQLRepository) GetById(ctx context.Context, id int64) (Model, error) {
	var tenant Tenant
	err := repo.DB.GetContext(
		ctx,
		&tenant,
		`
			SELECT id, objectId, tenantId, name, createdAt, updatedAt, deletedAt
			FROM tenant
			WHERE
				id = ? AND
				deletedAt IS NULL
		`,
		id,
	)
	if err != nil {
		switch err {
		case sql.ErrNoRows:
			return nil, service.NewRecordNotFoundError("Tenant", id)
		default:
			return nil, service.NewInternalError(fmt.Sprintf("Unable to get Tenant %d from mysql", id))
		}
	}

	return &tenant, nil
}

func (repo MySQLRepository) GetByTenantId(ctx context.Context, tenantId string) (Model, error) {
	var tenant Tenant
	err := repo.DB.GetContext(
		ctx,
		&tenant,
		`
			SELECT id, objectId, tenantId, name, createdAt, updatedAt, deletedAt
			FROM tenant
			WHERE
				tenantId = ? AND
				deletedAt IS NULL
		`,
		tenantId,
	)
	if err != nil {
		switch err {
		case sql.ErrNoRows:
			return nil, service.NewRecordNotFoundError("Tenant", tenantId)
		default:
			return nil, service.NewInternalError(fmt.Sprintf("Unable to get Tenant %s from mysql", tenantId))
		}
	}

	return &tenant, nil
}

func (repo MySQLRepository) List(ctx context.Context, listParams middleware.ListParams) ([]Model, error) {
	models := make([]Model, 0)
	tenants := make([]Tenant, 0)
	query := `
		SELECT id, objectId, tenantId, name, createdAt, updatedAt, deletedAt
		FROM tenant
		WHERE
			deletedAt IS NULL

	`
	replacements := []interface{}{}

	if listParams.Query != "" {
		searchTermReplacement := fmt.Sprintf("%%%s%%", listParams.Query)
		query = fmt.Sprintf("%s AND (tenantId LIKE ? OR name LIKE ?)", query)
		replacements = append(replacements, searchTermReplacement, searchTermReplacement)
	}

	if listParams.AfterId != "" {
		if listParams.AfterValue != nil {
			if listParams.SortOrder == middleware.SortOrderAsc {
				query = fmt.Sprintf("%s AND (%s > ? OR (tenantId > ? AND %s = ?))", query, listParams.SortBy, listParams.SortBy)
				replacements = append(replacements,
					listParams.AfterValue,
					listParams.AfterId,
					listParams.AfterValue,
				)
			} else {
				query = fmt.Sprintf("%s AND (%s < ? OR (tenantId < ? AND %s = ?))", query, listParams.SortBy, listParams.SortBy)
				replacements = append(replacements,
					listParams.AfterValue,
					listParams.AfterId,
					listParams.AfterValue,
				)
			}
		} else {
			if listParams.SortOrder == middleware.SortOrderAsc {
				query = fmt.Sprintf("%s AND tenantId > ?", query)
				replacements = append(replacements, listParams.AfterId)
			} else {
				query = fmt.Sprintf("%s AND tenantId < ?", query)
				replacements = append(replacements, listParams.AfterId)
			}
		}
	}

	if listParams.BeforeId != "" {
		if listParams.BeforeValue != nil {
			if listParams.SortOrder == middleware.SortOrderAsc {
				query = fmt.Sprintf("%s AND (%s < ? OR (tenantId < ? AND %s = ?))", query, listParams.SortBy, listParams.SortBy)
				replacements = append(replacements,
					listParams.BeforeValue,
					listParams.BeforeId,
					listParams.BeforeValue,
				)
			} else {
				query = fmt.Sprintf("%s AND (%s > ? OR (tenantId > ? AND %s = ?))", query, listParams.SortBy, listParams.SortBy)
				replacements = append(replacements,
					listParams.BeforeValue,
					listParams.BeforeId,
					listParams.BeforeValue,
				)
			}
		} else {
			if listParams.SortOrder == middleware.SortOrderAsc {
				query = fmt.Sprintf("%s AND tenantId < ?", query)
				replacements = append(replacements, listParams.AfterId)
			} else {
				query = fmt.Sprintf("%s AND tenantId > ?", query)
				replacements = append(replacements, listParams.AfterId)
			}
		}
	}

	if listParams.SortBy != "tenantId" {
		query = fmt.Sprintf("%s ORDER BY %s %s, tenantId %s LIMIT ?", query, listParams.SortBy, listParams.SortOrder, listParams.SortOrder)
		replacements = append(replacements, listParams.Limit)
	} else {
		query = fmt.Sprintf("%s ORDER BY tenantId %s LIMIT ?", query, listParams.SortOrder)
		replacements = append(replacements, listParams.Limit)
	}

	err := repo.DB.SelectContext(
		ctx,
		&tenants,
		query,
		replacements...,
	)
	if err != nil {
		switch err {
		case sql.ErrNoRows:
			return models, nil
		default:
			return models, service.NewInternalError("Unable to list tenants")
		}
	}

	for i := range tenants {
		models = append(models, &tenants[i])
	}

	return models, nil
}

func (repo MySQLRepository) UpdateByTenantId(ctx context.Context, tenantId string, model Model) error {
	_, err := repo.DB.ExecContext(
		ctx,
		`
			UPDATE tenant
			SET
				name = ?
			WHERE
				tenantId = ? AND
				deletedAt IS NULL
		`,
		model.GetName(),
		tenantId,
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Error updating tenant %d", model.GetID()))
	}

	return nil
}

func (repo MySQLRepository) DeleteByTenantId(ctx context.Context, tenantId string) error {
	_, err := repo.DB.ExecContext(
		ctx,
		`
			UPDATE tenant
			SET
				deletedAt = ?
			WHERE
				tenantId = ? AND
				deletedAt IS NULL
		`,
		time.Now().UTC(),
		tenantId,
	)
	if err != nil {
		switch err {
		case sql.ErrNoRows:
			return service.NewRecordNotFoundError("Tenant", tenantId)
		default:
			return err
		}
	}

	return nil
}
