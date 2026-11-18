package server

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/trisacrypto/daybreak/pkg/api/v1"
	"github.com/trisacrypto/daybreak/pkg/db"
	"github.com/trisacrypto/daybreak/pkg/errors"
	"github.com/trisacrypto/daybreak/pkg/web/scene"
	"go.rtnl.ai/gimlet/htmx"
	"go.rtnl.ai/ulid"
)

func (s *Server) ListLicenses(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err      error
		out      []*api.License
		licenses []*db.License
	)

	if licenses, err = s.db.ListLicenses(ctx); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to list licenses")
	}

	out = make([]*api.License, 0, len(licenses))
	for _, license := range licenses {
		out = append(out, api.NewLicense(license))
	}

	c.Negotiate(http.StatusOK, gin.Negotiate{
		Offered:  []string{binding.MIMEJSON, binding.MIMEHTML},
		HTMLName: "partials/licenses/list.html",
		HTMLData: scene.New(c).WithAPIData(out),
		JSONData: out,
	})
	return nil
}

func (s *Server) CreateLicense(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err     error
		in      *api.License
		license *db.License
	)

	in = &api.License{}
	if err = c.BindJSON(in); err != nil {
		return api.Error(http.StatusBadRequest, err, "could not parse request")
	}

	if err = in.Validate(c.Request.Method); err != nil {
		return api.Error(http.StatusUnprocessableEntity, err, "invalid license")
	}

	license = in.Model()
	if err = s.db.CreateLicense(ctx, license); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to create license")
	}

	if htmx.TriggerCreated(c, "license") {
		return nil
	}

	c.JSON(http.StatusCreated, api.NewLicense(license))
	return nil
}

func (s *Server) RetrieveLicense(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err       error
		licenseID ulid.ULID
		license   *db.License
		out       *api.License
	)

	if licenseID, err = ulid.Parse(c.Param("id")); err != nil {
		return api.Error(http.StatusNotFound, err, "invalid license ID")
	}

	if license, err = s.db.RetrieveLicense(ctx, licenseID); err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			return api.Error(http.StatusNotFound, err, "license not found")
		}
		return api.Error(http.StatusInternalServerError, err, "failed to retrieve license")
	}

	out = api.NewLicense(license)
	c.Negotiate(http.StatusOK, gin.Negotiate{
		Offered:  []string{binding.MIMEJSON, binding.MIMEHTML},
		HTMLName: "partials/licenses/detail.html",
		HTMLData: scene.New(c).WithAPIData(out),
		JSONData: out,
	})
	return nil
}

func (s *Server) LicenseCredentials(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err       error
		licenseID ulid.ULID
		license   *db.License
		out       *api.LicenseCredentials
	)

	if licenseID, err = ulid.Parse(c.Param("id")); err != nil {
		return api.Error(http.StatusNotFound, err, "invalid license ID")
	}

	if license, err = s.db.RetrieveLicense(ctx, licenseID); err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			return api.Error(http.StatusNotFound, err, "license not found")
		}
		return api.Error(http.StatusInternalServerError, err, "failed to retrieve license")
	}

	out = api.NewLicenseCredentials(license)
	c.Negotiate(http.StatusOK, gin.Negotiate{
		Offered:  []string{binding.MIMEJSON, binding.MIMEHTML},
		HTMLName: "partials/licenses/credentials.html",
		HTMLData: scene.New(c).WithAPIData(out),
		JSONData: out,
	})
	return nil
}

func (s *Server) UpdateLicense(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err       error
		in        *api.License
		licenseID ulid.ULID
		license   *db.License
	)

	if licenseID, err = ulid.Parse(c.Param("id")); err != nil {
		return api.Error(http.StatusNotFound, err, "invalid license ID")
	}

	in = &api.License{}
	if err = c.BindJSON(in); err != nil {
		return api.Error(http.StatusBadRequest, err, "could not parse request")
	}

	if err = in.Validate(c.Request.Method); err != nil {
		return api.Error(http.StatusUnprocessableEntity, err, "invalid license")
	}

	license = in.Model()
	license.ID = licenseID

	// Start a transaction to update the license
	var tx *db.Tx
	if tx, err = s.db.Begin(ctx, &sql.TxOptions{ReadOnly: false}); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to start transaction")
	}
	defer tx.Rollback()

	if err = tx.UpdateLicense(license); err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			return api.Error(http.StatusNotFound, err, "license not found")
		}
		return api.Error(http.StatusInternalServerError, err, "failed to update license")
	}

	// Retrieve the updated company
	if license, err = tx.RetrieveLicense(licenseID); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to retrieve license after update")
	}

	if err = tx.Commit(); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to commit transaction")
	}

	if htmx.TriggerUpdated(c, "license") {
		return nil
	}

	c.JSON(http.StatusOK, api.NewLicense(license))
	return nil
}

func (s *Server) DeleteLicense(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err       error
		licenseID ulid.ULID
	)

	if licenseID, err = ulid.Parse(c.Param("id")); err != nil {
		return api.Error(http.StatusNotFound, err, "invalid license ID")
	}

	if err = s.db.DeleteLicense(ctx, licenseID); err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			return api.Error(http.StatusNotFound, err, "license not found")
		}
		return api.Error(http.StatusInternalServerError, err, "failed to delete license")
	}

	if htmx.TriggerDeleted(c, "license") {
		return nil
	}

	c.JSON(http.StatusNoContent, nil)
	return nil
}
