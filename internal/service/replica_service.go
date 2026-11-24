package service

import (
	"context"
	"fmt"
	"time"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
	"github.com/mbagusaditya/scrobbles-cli/internal/repository"
)

// ReplicaService mengorkestrasi sinkronisasi data dua arah antara repository lokal dan remote
// menggunakan strategi Watermark + Scoped Master agar tidak pernah melakukan full table scan
// pada sinkronisasi harian.
type ReplicaService struct {
	localRepo  *repository.ReplicaRepository
	remoteRepo *repository.ReplicaRepository
}

func NewReplicaService(localRepo, remoteRepo *repository.ReplicaRepository) *ReplicaService {
	return &ReplicaService{
		localRepo:  localRepo,
		remoteRepo: remoteRepo,
	}
}

// Push mengirim data baru/terbaru dari SQLite lokal ke Turso Cloud.
func (s *ReplicaService) Push(ctx context.Context, opts model.ReplicaOptions, onProgress func(model.ReplicaProgress)) (*model.ReplicaResult, error) {
	return s.transfer(ctx, model.DirectionPush, s.localRepo, s.remoteRepo, opts, onProgress)
}

// Pull menarik data baru/terbaru dari Turso Cloud ke SQLite lokal.
func (s *ReplicaService) Pull(ctx context.Context, opts model.ReplicaOptions, onProgress func(model.ReplicaProgress)) (*model.ReplicaResult, error) {
	return s.transfer(ctx, model.DirectionPull, s.remoteRepo, s.localRepo, opts, onProgress)
}

// InspectDatabases mengambil ringkasan jumlah entitas pada SQLite lokal dan Turso Cloud.
func (s *ReplicaService) InspectDatabases(ctx context.Context) (local, remote model.DBStatsSummary, err error) {
	local, err = s.localRepo.GetSummary(ctx)
	if err != nil {
		return local, remote, fmt.Errorf("gagal membaca ringkasan SQLite lokal: %w", err)
	}
	remote, err = s.remoteRepo.GetSummary(ctx)
	if err != nil {
		return local, remote, fmt.Errorf("gagal membaca ringkasan Turso remote: %w", err)
	}
	return local, remote, nil
}

func (s *ReplicaService) transfer(
	ctx context.Context,
	dir model.ReplicaDirection,
	src *repository.ReplicaRepository,
	dst *repository.ReplicaRepository,
	opts model.ReplicaOptions,
	onProgress func(model.ReplicaProgress),
) (*model.ReplicaResult, error) {
	start := time.Now()
	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = 500
	}

	res := &model.ReplicaResult{Direction: dir}

	// 1. Tentukan watermark incremental (kecuali opts.Full = true) dan tarik log kandidat dari sumber
	var minPlayedAt int64
	if !opts.Full {
		maxDstPlayed, err := dst.GetMaxPlayedAt(ctx)
		if err != nil {
			return nil, fmt.Errorf("gagal membaca watermark tujuan: %w", err)
		}
		if maxDstPlayed > 86400 {
			// Mundur 24 jam sebagai safety window untuk menangkap scrobble terlambat
			minPlayedAt = maxDstPlayed - 86400
		}
	}

	srcLogs, err := src.ListScrobbleLogsSince(ctx, minPlayedAt)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca scrobble_logs dari sumber: %w", err)
	}

	// Short-circuit: jika tidak ada log dalam rentang tersebut, tidak perlu menyentuh tabel master
	if len(srcLogs) == 0 {
		res.Duration = time.Since(start)
		return res, nil
	}

	// 2. Kumpulkan track_id unik yang direferensikan oleh log baru saja (Scoped Master)
	trackIDSet := make(map[string]struct{})
	for _, l := range srcLogs {
		if l.TrackID != "" {
			trackIDSet[l.TrackID] = struct{}{}
		}
	}

	srcTracks, err := src.ListTracksByIDs(ctx, keysOfSet(trackIDSet))
	if err != nil {
		return nil, fmt.Errorf("gagal membaca tracks terkait dari sumber: %w", err)
	}

	// 3. Kumpulkan artist_id unik yang direferensikan oleh tracks terkait
	artistIDSet := make(map[string]struct{}, len(srcTracks))
	for _, t := range srcTracks {
		if t.ArtistID != "" {
			artistIDSet[t.ArtistID] = struct{}{}
		}
	}

	srcArtists, err := src.ListArtistsByIDs(ctx, keysOfSet(artistIDSet))
	if err != nil {
		return nil, fmt.Errorf("gagal membaca artists terkait dari sumber: %w", err)
	}

	// 4. Sinkronisasi scoped artists -> bangun peta sourceArtistID -> targetArtistID
	artistIDMap, insertedArtists, err := s.syncScopedArtists(ctx, dst, srcArtists, batchSize, onProgress)
	if err != nil {
		return nil, fmt.Errorf("gagal sinkronisasi artists: %w", err)
	}
	res.ArtistsProcessed = len(srcArtists)
	res.ArtistsInserted = insertedArtists

	// 5. Sinkronisasi scoped tracks -> bangun peta sourceTrackID -> targetTrackID
	trackIDMap, insertedTracks, err := s.syncScopedTracks(ctx, dst, srcTracks, artistIDMap, batchSize, onProgress)
	if err != nil {
		return nil, fmt.Errorf("gagal sinkronisasi tracks: %w", err)
	}
	res.TracksProcessed = len(srcTracks)
	res.TracksInserted = insertedTracks

	// 6. Sinkronisasi scrobble_logs dengan track_id yang sudah di-remap
	insertedLogs, err := s.syncLogs(ctx, dst, srcLogs, trackIDMap, batchSize, onProgress)
	if err != nil {
		return nil, fmt.Errorf("gagal sinkronisasi scrobble_logs: %w", err)
	}
	res.LogsProcessed = len(srcLogs)
	res.LogsInserted = insertedLogs
	res.Duration = time.Since(start)

	return res, nil
}

func (s *ReplicaService) syncScopedArtists(
	ctx context.Context,
	dst *repository.ReplicaRepository,
	srcArtists []model.ReplicaArtist,
	batchSize int,
	onProgress func(model.ReplicaProgress),
) (map[string]string, int, error) {
	sourceToTarget := make(map[string]string, len(srcArtists))
	if len(srcArtists) == 0 {
		return sourceToTarget, 0, nil
	}

	names := make([]string, 0, len(srcArtists))
	for _, a := range srcArtists {
		names = append(names, a.Name)
	}

	dstNameToID, err := dst.GetArtistNameMapByNames(ctx, names)
	if err != nil {
		return nil, 0, err
	}

	inserted := 0
	total := len(srcArtists)

	for i := 0; i < total; i += batchSize {
		end := minInt(i+batchSize, total)
		var batch []model.ReplicaArtist

		for _, a := range srcArtists[i:end] {
			if existingID, ok := dstNameToID[a.Name]; ok {
				sourceToTarget[a.ID] = existingID
				if a.MBID != "" || a.ImageURL != "" {
					a.ID = existingID
					batch = append(batch, a)
				}
				continue
			}

			dstNameToID[a.Name] = a.ID
			sourceToTarget[a.ID] = a.ID
			batch = append(batch, a)
			inserted++
		}

		if err := dst.UpsertArtistsBatch(ctx, batch); err != nil {
			return nil, 0, err
		}

		if onProgress != nil {
			onProgress(model.ReplicaProgress{Stage: "artists", Processed: end, Total: total})
		}
	}

	return sourceToTarget, inserted, nil
}

func (s *ReplicaService) syncScopedTracks(
	ctx context.Context,
	dst *repository.ReplicaRepository,
	srcTracks []model.ReplicaTrack,
	artistIDMap map[string]string,
	batchSize int,
	onProgress func(model.ReplicaProgress),
) (map[string]string, int, error) {
	sourceToTarget := make(map[string]string, len(srcTracks))
	if len(srcTracks) == 0 {
		return sourceToTarget, 0, nil
	}

	targetArtistIDSet := make(map[string]struct{}, len(artistIDMap))
	for _, targetID := range artistIDMap {
		targetArtistIDSet[targetID] = struct{}{}
	}

	dstTrackMap, err := dst.GetTrackNaturalMapByArtists(ctx, keysOfSet(targetArtistIDSet))
	if err != nil {
		return nil, 0, err
	}

	inserted := 0
	total := len(srcTracks)

	for i := 0; i < total; i += batchSize {
		end := minInt(i+batchSize, total)
		var toInsert, toEnrich []model.ReplicaTrack

		for _, t := range srcTracks[i:end] {
			targetArtistID, ok := artistIDMap[t.ArtistID]
			if !ok {
				continue
			}

			key := model.TrackNaturalKey{
				ArtistID: targetArtistID,
				Title:    t.Title,
				Album:    t.AlbumName,
			}

			if existingTrackID, exists := dstTrackMap[key]; exists {
				sourceToTarget[t.ID] = existingTrackID
				if t.HasDur || t.CoverURL != "" {
					t.ID = existingTrackID
					toEnrich = append(toEnrich, t)
				}
				continue
			}

			t.ArtistID = targetArtistID
			dstTrackMap[key] = t.ID
			sourceToTarget[t.ID] = t.ID
			toInsert = append(toInsert, t)
			inserted++
		}

		if err := dst.UpsertTracksBatch(ctx, toInsert, toEnrich); err != nil {
			return nil, 0, err
		}

		if onProgress != nil {
			onProgress(model.ReplicaProgress{Stage: "tracks", Processed: end, Total: total})
		}
	}

	return sourceToTarget, inserted, nil
}

func (s *ReplicaService) syncLogs(
	ctx context.Context,
	dst *repository.ReplicaRepository,
	srcLogs []model.ReplicaScrobbleLog,
	trackIDMap map[string]string,
	batchSize int,
	onProgress func(model.ReplicaProgress),
) (int, error) {
	inserted := 0
	total := len(srcLogs)

	for i := 0; i < total; i += batchSize {
		end := minInt(i+batchSize, total)
		batch := make([]model.ReplicaScrobbleLog, 0, end-i)

		for _, l := range srcLogs[i:end] {
			if l.TrackID != "" {
				l.TrackID = trackIDMap[l.TrackID]
			}
			batch = append(batch, l)
		}

		affected, err := dst.UpsertScrobbleLogsBatch(ctx, batch)
		if err != nil {
			return 0, err
		}
		inserted += affected

		if onProgress != nil {
			onProgress(model.ReplicaProgress{Stage: "scrobble_logs", Processed: end, Total: total})
		}
	}

	return inserted, nil
}

func keysOfSet(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	return keys
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
