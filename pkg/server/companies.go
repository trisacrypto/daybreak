package server

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"github.com/trisacrypto/daybreak/pkg/api/v1"
	"github.com/trisacrypto/daybreak/pkg/db"
	"github.com/trisacrypto/daybreak/pkg/errors"
	"github.com/trisacrypto/daybreak/pkg/web/scene"
	"go.rtnl.ai/gimlet/htmx"
)

func (s *Server) ListCompanies(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err       error
		out       []*api.Company
		companies []*db.Company
	)

	if companies, err = s.db.ListCompanies(ctx); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to list companies")
	}

	out = make([]*api.Company, 0, len(companies))
	for _, company := range companies {
		out = append(out, api.NewCompany(company))
	}

	c.Negotiate(http.StatusOK, gin.Negotiate{
		Offered:  []string{binding.MIMEJSON, binding.MIMEHTML},
		HTMLName: "partials/companies/list.html",
		HTMLData: scene.New(c).WithAPIData(out),
		JSONData: out,
	})
	return nil
}

func (s *Server) CreateCompany(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err     error
		in      *api.Company
		company *db.Company
	)

	in = &api.Company{}
	if err = c.BindJSON(in); err != nil {
		return api.Error(http.StatusBadRequest, err, "could not parse request")
	}

	if err = in.Validate(c.Request.Method); err != nil {
		return api.Error(http.StatusUnprocessableEntity, err, "invalid company")
	}

	company = in.Model()
	contacts := make([]*db.Contact, 0, len(in.Contacts))
	for _, contact := range in.Contacts {
		contacts = append(contacts, contact.Model())
	}

	if herr := s.createCompany(ctx, company, contacts); herr != nil {
		return herr
	}

	// TODO: change to trigger created redirect to the company detail page.
	if htmx.TriggerCreated(c, "company") {
		return nil
	}

	out := api.NewCompany(company)
	for _, contact := range contacts {
		out.Contacts = append(out.Contacts, api.NewContact(contact))
	}

	c.JSON(http.StatusCreated, out)
	return nil
}

// createCompany atomically creates the company and any initial contacts, setting the
// first contact as the company's primary contact.
func (s *Server) createCompany(ctx context.Context, company *db.Company, contacts []*db.Contact) *errors.HTTP {
	var (
		tx  *db.Tx
		err error
	)

	if tx, err = s.db.Begin(ctx, &sql.TxOptions{ReadOnly: false}); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to begin transaction")
	}
	defer tx.Rollback()

	if err = tx.CreateCompany(company); err != nil {
		if errors.Is(err, errors.ErrAlreadyExists) {
			return api.Error(http.StatusConflict, err, "company already exists")
		}
		return api.Error(http.StatusInternalServerError, err, "failed to create company")
	}

	for _, contact := range contacts {
		contact.CompanyID = company.ID
		if err = tx.CreateContact(contact); err != nil {
			if errors.Is(err, errors.ErrAlreadyExists) {
				return api.Error(http.StatusConflict, err, "contact already exists")
			}
			return api.Error(http.StatusInternalServerError, err, "failed to create contact")
		}
	}

	// The primary contact can only reference a contact row once it exists.
	if len(contacts) > 0 {
		company.PrimaryContact = uuid.NullUUID{UUID: contacts[0].ID, Valid: true}
		if err = tx.UpdateCompany(company); err != nil {
			return api.Error(http.StatusInternalServerError, err, "failed to set primary contact")
		}
	}

	if err = tx.Commit(); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to commit transaction")
	}
	return nil
}

func (s *Server) RetrieveCompany(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err       error
		companyID uuid.UUID
		company   *db.Company
		out       *api.Company
	)

	if companyID, err = uuid.Parse(c.Param("id")); err != nil {
		return api.Error(http.StatusNotFound, err, "invalid company ID")
	}

	if company, err = s.db.RetrieveCompany(ctx, companyID); err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			return api.Error(http.StatusNotFound, err, "company not found")
		}
		return api.Error(http.StatusInternalServerError, err, "failed to retrieve company")
	}

	out = api.NewCompany(company)
	c.Negotiate(http.StatusOK, gin.Negotiate{
		Offered:  []string{binding.MIMEJSON, binding.MIMEHTML},
		HTMLName: "partials/companies/detail.html",
		HTMLData: scene.New(c).WithAPIData(out),
		JSONData: out,
	})
	return nil
}

func (s *Server) UpdateCompany(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err       error
		in        *api.Company
		companyID uuid.UUID
		company   *db.Company
	)

	if companyID, err = uuid.Parse(c.Param("id")); err != nil {
		return api.Error(http.StatusNotFound, err, "invalid company ID")
	}

	in = &api.Company{}
	if err = c.BindJSON(in); err != nil {
		return api.Error(http.StatusBadRequest, err, "could not parse request")
	}

	if err = in.Validate(c.Request.Method); err != nil {
		return api.Error(http.StatusUnprocessableEntity, err, "invalid company")
	}

	company = in.Model()
	company.ID = companyID

	// Start a transaction to update the company
	var tx *db.Tx
	if tx, err = s.db.Begin(ctx, &sql.TxOptions{ReadOnly: false}); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to start transaction")
	}
	defer tx.Rollback()

	if err = tx.UpdateCompany(company); err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			return api.Error(http.StatusNotFound, err, "company not found")
		}

		if errors.Is(err, errors.ErrAlreadyExists) {
			return api.Error(http.StatusConflict, err, "company already exists")
		}

		return api.Error(http.StatusInternalServerError, err, "failed to update company")
	}

	// Retrieve the updated company
	if company, err = tx.RetrieveCompany(company.ID); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to retrieve company after update")
	}

	if err = tx.Commit(); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to commit transaction")
	}

	if htmx.TriggerUpdated(c, "company") {
		return nil
	}

	c.JSON(http.StatusOK, api.NewCompany(company))
	return nil
}

func (s *Server) DeleteCompany(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err       error
		companyID uuid.UUID
	)

	if companyID, err = uuid.Parse(c.Param("id")); err != nil {
		return api.Error(http.StatusNotFound, err, "invalid company ID")
	}

	if err = s.db.DeleteCompany(ctx, companyID); err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			return api.Error(http.StatusNotFound, err, "company not found")
		}
		return api.Error(http.StatusInternalServerError, err, "failed to delete company")
	}

	if htmx.TriggerDeleted(c, "company") {
		return nil
	}

	c.JSON(http.StatusNoContent, nil)
	return nil
}
