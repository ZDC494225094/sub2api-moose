package repository

import (
	"context"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (s *S3ImageStorage) ListAssets(ctx context.Context, page, size int, search string) (*service.ObjectStorageAssetPage, error) {
	if s.db == nil {
		return nil, fmt.Errorf("asset database is unavailable")
	}
	result := &service.ObjectStorageAssetPage{Items: []service.ObjectStorageAsset{}}
	filter := ` FROM object_storage_assets a LEFT JOIN users u ON u.id=a.user_id
        WHERE ($1='' OR u.email ILIKE $2 OR a.object_key ILIKE $2 OR a.user_id::text=$1)`
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*)`+filter, search, "%"+search+"%").Scan(&result.Total); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT a.id,a.user_id,COALESCE(u.email,''),a.bucket,a.object_key,a.content_type,a.bytes,a.uploaded_at`+filter+` ORDER BY a.uploaded_at DESC,a.id DESC LIMIT $3 OFFSET $4`, search, "%"+search+"%", size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item service.ObjectStorageAsset
		if err := rows.Scan(&item.ID, &item.UserID, &item.UserEmail, &item.Bucket, &item.ObjectKey, &item.ContentType, &item.Bytes, &item.UploadedAt); err != nil {
			return nil, err
		}
		if item.Bucket == s.bucket {
			item.URL, err = s.assetURL(ctx, item.ObjectKey)
			if err != nil {
				return nil, err
			}
		}
		result.Items = append(result.Items, item)
	}
	return result, rows.Err()
}
