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
	"go.rtnl.ai/x/validation"
)

func (s *Server) ListContacts(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err      error
		out      []*api.Contact
		contacts []*db.Contact
	)

	if contacts, err = s.db.ListContacts(ctx); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to list contacts")
	}

	out = make([]*api.Contact, 0, len(contacts))
	for _, contact := range contacts {
		out = append(out, api.NewContact(contact))
	}

	c.Negotiate(http.StatusOK, gin.Negotiate{
		Offered:  []string{binding.MIMEJSON, binding.MIMEHTML},
		HTMLName: "partials/contacts/list.html",
		HTMLData: scene.New(c).WithAPIData(out),
		JSONData: out,
	})
	return nil
}

func (s *Server) ListCompanyContacts(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err       error
		out       []*api.Contact
		contacts  []*db.Contact
		companyID uuid.UUID
	)

	if companyID, err = uuid.Parse(c.Param("id")); err != nil {
		return api.Error(http.StatusNotFound, err, "invalid company ID")
	}

	if contacts, err = s.db.ListCompanyContacts(ctx, companyID); err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			return api.Error(http.StatusNotFound, err, "company not found")
		}
		return api.Error(http.StatusInternalServerError, err, "failed to list contacts")
	}

	out = make([]*api.Contact, 0, len(contacts))
	for _, contact := range contacts {
		out = append(out, api.NewContact(contact))
	}

	c.Negotiate(http.StatusOK, gin.Negotiate{
		Offered:  []string{binding.MIMEJSON, binding.MIMEHTML},
		HTMLName: "partials/contacts/list.html",
		HTMLData: scene.New(c).WithAPIData(out),
		JSONData: out,
	})
	return nil
}

func (s *Server) CreateContact(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err     error
		in      *api.Contact
		contact *db.Contact
	)

	in = &api.Contact{}
	if err = c.BindJSON(in); err != nil {
		return api.Error(http.StatusBadRequest, err, "could not parse request")
	}

	if err = in.Validate(c.Request.Method); err != nil {
		return api.Error(http.StatusUnprocessableEntity, err, "invalid contact")
	}

	contact = in.Model()
	if contact.CompanyID == uuid.Nil {
		return api.Error(http.StatusUnprocessableEntity, validation.Required("company_id"), "company ID is required")
	}

	if herr := s.createContact(ctx, contact); herr != nil {
		return herr
	}

	if htmx.TriggerCreated(c, "contact") {
		return nil
	}

	c.JSON(http.StatusCreated, api.NewContact(contact))
	return nil
}

func (s *Server) CreateCompanyContact(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err       error
		in        *api.Contact
		contact   *db.Contact
		companyID uuid.UUID
	)

	if companyID, err = uuid.Parse(c.Param("id")); err != nil {
		return api.Error(http.StatusNotFound, err, "invalid company ID")
	}

	if companyID == uuid.Nil {
		return api.Error(http.StatusBadRequest, validation.Required("company_id"), "invalid company ID")
	}

	in = &api.Contact{}
	if err = c.BindJSON(in); err != nil {
		return api.Error(http.StatusBadRequest, err, "could not parse request")
	}

	if err = in.Validate(c.Request.Method); err != nil {
		return api.Error(http.StatusUnprocessableEntity, err, "invalid contact")
	}

	contact = in.Model()
	if contact.CompanyID != uuid.Nil && contact.CompanyID != companyID {
		return api.Error(http.StatusUnprocessableEntity, validation.Invalid("company_id", "company ID does not match"), "company ID does not match")
	}

	contact.CompanyID = companyID
	if herr := s.createContact(ctx, contact); herr != nil {
		return herr
	}

	if htmx.TriggerCreated(c, "contact") {
		return nil
	}

	c.JSON(http.StatusCreated, api.NewContact(contact))
	return nil
}

func (s *Server) createContact(ctx context.Context, contact *db.Contact) *errors.HTTP {
	var (
		tx  *db.Tx
		err error
	)

	if tx, err = s.db.Begin(ctx, &sql.TxOptions{ReadOnly: false}); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to begin transaction")
	}
	defer tx.Rollback()

	// Ensure the company exists
	if _, err = tx.RetrieveCompany(contact.CompanyID); err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			return api.Error(http.StatusNotFound, err, "company not found")
		}
		return api.Error(http.StatusInternalServerError, err, "failed to retrieve company")
	}

	if err = tx.CreateContact(contact); err != nil {
		if errors.Is(err, errors.ErrAlreadyExists) {
			return api.Error(http.StatusConflict, err, "contact already exists")
		}
		return api.Error(http.StatusInternalServerError, err, "failed to create contact")
	}

	if err = tx.Commit(); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to commit transaction")
	}

	return nil
}

func (s *Server) RetrieveContact(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err       error
		contactID uuid.UUID
		contact   *db.Contact
		out       *api.Contact
	)

	if contactID, err = uuid.Parse(c.Param("id")); err != nil {
		return api.Error(http.StatusNotFound, err, "invalid contact ID")
	}

	if contact, err = s.db.RetrieveContact(ctx, contactID); err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			return api.Error(http.StatusNotFound, err, "contact not found")
		}
		return api.Error(http.StatusInternalServerError, err, "failed to retrieve contact")
	}

	out = api.NewContact(contact)
	c.Negotiate(http.StatusOK, gin.Negotiate{
		Offered:  []string{binding.MIMEJSON, binding.MIMEHTML},
		HTMLName: "partials/contacts/detail.html",
		HTMLData: scene.New(c).WithAPIData(out),
		JSONData: out,
	})
	return nil
}

func (s *Server) UpdateContact(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err       error
		in        *api.Contact
		contactID uuid.UUID
		contact   *db.Contact
		out       *api.Contact
	)

	if contactID, err = uuid.Parse(c.Param("id")); err != nil {
		return api.Error(http.StatusNotFound, err, "invalid contact ID")
	}

	in = &api.Contact{}
	if err = c.BindJSON(in); err != nil {
		return api.Error(http.StatusBadRequest, err, "could not parse request")
	}

	if err = in.Validate(c.Request.Method); err != nil {
		return api.Error(http.StatusUnprocessableEntity, err, "invalid contact")
	}

	contact = in.Model()
	contact.ID = contactID

	var tx *db.Tx
	if tx, err = s.db.Begin(ctx, &sql.TxOptions{ReadOnly: false}); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to begin transaction")
	}
	defer tx.Rollback()

	if err = tx.UpdateContact(contact); err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			return api.Error(http.StatusNotFound, err, "contact not found")
		}
		if errors.Is(err, errors.ErrAlreadyExists) {
			return api.Error(http.StatusConflict, err, "contact already exists")
		}
		if errors.Is(err, errors.ErrForeignKeyViolation) {
			return api.Error(http.StatusNotFound, err, "company not found")
		}
		return api.Error(http.StatusInternalServerError, err, "failed to update contact")
	}

	if contact, err = tx.RetrieveContact(contactID); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to retrieve contact after update")
	}

	if err = tx.Commit(); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to commit transaction")
	}

	out = api.NewContact(contact)
	c.Negotiate(http.StatusOK, gin.Negotiate{
		Offered:  []string{binding.MIMEJSON, binding.MIMEHTML},
		HTMLName: "partials/contacts/detail.html",
		HTMLData: scene.New(c).WithAPIData(out),
		JSONData: out,
	})
	return nil
}

func (s *Server) DeleteContact(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err       error
		contactID uuid.UUID
	)

	if contactID, err = uuid.Parse(c.Param("id")); err != nil {
		return api.Error(http.StatusNotFound, err, "invalid contact ID")
	}

	if err = s.db.DeleteContact(ctx, contactID); err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			return api.Error(http.StatusNotFound, err, "contact not found")
		}
		return api.Error(http.StatusInternalServerError, err, "failed to delete contact")
	}

	if htmx.TriggerDeleted(c, "contact") {
		return nil
	}

	c.JSON(http.StatusNoContent, nil)
	return nil
}
