package server

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trisacrypto/daybreak/pkg/api/v1"
	"github.com/trisacrypto/daybreak/pkg/auth"
	"github.com/trisacrypto/daybreak/pkg/db"
	"github.com/trisacrypto/daybreak/pkg/errors"
	"go.rtnl.ai/ulid"
	"go.rtnl.ai/x/rlog"
)

const (
	HeaderIfModifiedSince = "If-Modified-Since"
	HeaderCacheControl    = "Cache-Control"
	CacheControlValue     = "private, no-store"
	HeaderLastModified    = "Last-Modified"
)

// Returns a list of the modified and deleted companies and contacts since the client's
// last sync time. If the client doesn't have a last sync time it will return the IDs
// of the entire database with no deleted IDs.
func (s *Server) DaybreakSync(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		ok           bool
		tx           *db.Tx
		err          error
		lastSync     time.Time
		lastModified time.Time
		out          *api.SyncResponse
	)

	// Record the start time of the sync operation to be used as a fallback for
	// lastModified if it hasn't been set in the database. This must happen before the
	// transaction is started to avoid a race condition with a write after the read.
	syncStart := time.Now().UTC().Truncate(time.Second)

	if tx, err = s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true}); err != nil {
		return api.Error(http.StatusInternalServerError, err, "could not begin sync operation")
	}
	defer tx.Rollback()

	if lastModified, err = tx.LastModifiedHTTP(); err != nil {
		return api.Error(http.StatusInternalServerError, err, "could not begin sync operation")
	}

	if lastSync, ok = ifModifiedSince(c); ok {
		// If the client's last sync time is after the database's last modified time,
		// we can return a 304 Not Modified response along with cache-control headers.
		if !lastModified.IsZero() && !lastModified.After(lastSync) {
			c.Header(HeaderCacheControl, CacheControlValue)
			c.Header(HeaderLastModified, lastModified.Format(http.TimeFormat))
			c.Status(http.StatusNotModified)
			return nil
		}
	}

	if lastModified.IsZero() {
		lastModified = syncStart
	}

	out = &api.SyncResponse{
		LastModified: lastModified,
	}

	// Return all of the companies that have been modified since the client's last sync time.
	// cSpell:ignore itera iterb iterc
	var itera *db.Iterator[db.Modified, *db.Modified]
	if itera, err = tx.ListModifiedCompanies(lastSync); err != nil {
		return api.Error(http.StatusInternalServerError, err, "could not complete sync operation")
	}

	if out.Companies, err = api.NewModifiedList(itera); err != nil {
		return api.Error(http.StatusInternalServerError, err, "could not complete sync operation")
	}

	// Return all of the contacts that have been modified since the client's last sync time.
	if itera, err = tx.ListModifiedContacts(lastSync); err != nil {
		return api.Error(http.StatusInternalServerError, err, "could not complete sync operation")
	}

	if out.Contacts, err = api.NewModifiedList(itera); err != nil {
		return api.Error(http.StatusInternalServerError, err, "could not complete sync operation")
	}

	// Only send deleted companies if the last sync header is present.
	if ok {
		// Return all of the companies that have been deleted since the client's last sync time.
		var iterb *db.Iterator[db.DeletedCompany, *db.DeletedCompany]
		if iterb, err = tx.ListDeletedCompanies(lastSync); err != nil {
			return api.Error(http.StatusInternalServerError, err, "could not complete sync operation")
		}

		if out.DeletedCompanies, err = api.NewModifiedList(iterb); err != nil {
			return api.Error(http.StatusInternalServerError, err, "could not complete sync operation")
		}

		// Return all of the contacts that have been deleted since the client's last sync time.
		var iterc *db.Iterator[db.DeletedContact, *db.DeletedContact]
		if iterc, err = tx.ListDeletedContacts(lastSync, uuid.Nil); err != nil {
			return api.Error(http.StatusInternalServerError, err, "could not complete sync operation")
		}

		if out.DeletedContacts, err = api.NewModifiedList(iterc); err != nil {
			return api.Error(http.StatusInternalServerError, err, "could not complete sync operation")
		}
	}

	// Add cache-control headers to the response.
	c.Header(HeaderCacheControl, CacheControlValue)
	c.Header(HeaderLastModified, lastModified.Format(http.TimeFormat))

	// Return the sync response.
	c.JSON(http.StatusOK, out)

	// Send a message to the access writer to update the syncs counter.
	licenseID := auth.GetLicenseID(c)
	if !licenseID.IsZero() {
		go s.updateSyncs(licenseID)
	}

	return nil
}

func (s *Server) updateSyncs(licenseID ulid.ULID) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := s.db.IncrementLicenseSyncs(ctx, licenseID); err != nil {
		rlog.Error("could not increment license syncs", "error", err)
	}
}

func (s *Server) DaybreakDownload(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		ok           bool
		err          error
		lastSync     time.Time
		lastModified time.Time
	)

	// Record the start time of the download operation to be used as a fallback for
	// lastModified if it hasn't been set in the database. This must happen before the
	// transaction is started to avoid a race condition with a write after the read.
	syncStart := time.Now().UTC().Truncate(time.Second)

	// Read last modified in its own transaction. There is a minor race condition here
	// where a write could occur after the read but before the response is sent. The
	// impact of this is minimal since the download operation is the entire database
	// and does not depend on the last modified timestamp.
	if lastModified, err = s.db.LastModifiedHTTP(ctx); err != nil {
		return api.Error(http.StatusInternalServerError, err, "could not begin sync operation")
	}

	if lastSync, ok = ifModifiedSince(c); ok {
		// If the client's last sync time is after the database's last modified time,
		// we can return a 304 Not Modified response.
		if !lastModified.IsZero() && !lastModified.After(lastSync) {
			// Add cache-control headers to the response.
			c.Header(HeaderLastModified, lastModified.Format(http.TimeFormat))
			c.Header(HeaderCacheControl, CacheControlValue)

			c.Status(http.StatusNotModified)
			return nil
		}
	}

	if lastModified.IsZero() {
		lastModified = syncStart
	}

	// Begin the daybreak download operation.

	// Send a message to the access writer to update the downloads counter.
	licenseID := auth.GetLicenseID(c)
	if !licenseID.IsZero() {
		go s.updateDownloads(licenseID)
	}

	// Add cache-control headers to the response.
	c.Header(HeaderLastModified, lastModified.Format(http.TimeFormat))
	c.Header(HeaderCacheControl, CacheControlValue)
	c.JSON(http.StatusOK, gin.H{"lastModified": lastModified})
	return nil
}

func (s *Server) updateDownloads(licenseID ulid.ULID) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := s.db.IncrementLicenseDownloads(ctx, licenseID); err != nil {
		rlog.Error("could not increment license downloads", "error", err)
	}
}

func ifModifiedSince(c *gin.Context) (time.Time, bool) {
	// cSpell:ignore hval
	if hval := c.GetHeader(HeaderIfModifiedSince); hval != "" {
		if ts, err := http.ParseTime(hval); err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
}
