package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trisacrypto/daybreak/pkg/api/v1"
	"github.com/trisacrypto/daybreak/pkg/errors"
)

//============================================================================
// Test Status
//============================================================================

func TestClient_Status(t *testing.T) {
	t.Run("Ok", func(t *testing.T) {
		client := Serve(t, http.MethodGet, "/v1/status", func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusOK, &api.StatusReply{Status: "ok", Version: "1.0.0", Uptime: "1m32s"})
			return nil
		})

		reply, err := client.Status(context.Background())
		require.NoError(t, err)
		require.Equal(t, "ok", reply.Status)
		require.Equal(t, "1.0.0", reply.Version)
		require.Equal(t, "1m32s", reply.Uptime)
	})

	t.Run("Maintenance", func(t *testing.T) {
		client := Serve(t, http.MethodGet, "/v1/status", func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusServiceUnavailable, &api.StatusReply{Status: "maintenance", Version: "1.0.0", Uptime: "1m32s"})
			return nil
		})

		reply, err := client.Status(context.Background())
		require.NoError(t, err)
		require.Equal(t, "maintenance", reply.Status)
		require.Equal(t, "1.0.0", reply.Version)
		require.Equal(t, "1m32s", reply.Uptime)
	})

	t.Run("Error", func(t *testing.T) {
		client := Serve(t, http.MethodGet, "/v1/status", func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return nil
		})

		reply, err := client.Status(context.Background())
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusNotFound, err.(*api.StatusError).Code)
		require.Equal(t, "Not Found\n", err.(*api.StatusError).Reply.Error)
	})

	t.Run("NoConnection", func(t *testing.T) {
		client, _ := api.New("https://localhost:31232")
		reply, err := client.Status(context.Background())
		require.Nil(t, reply)
		require.Error(t, err)
		require.True(t, errors.Is(err, syscall.ECONNREFUSED))
	})
}

//============================================================================
// Test Companies
//============================================================================

// companyFixtures returns a fresh copy of the companies used across the client
// tests so that individual tests can safely modify them.
func companyFixtures() []*api.Company {
	return []*api.Company{
		{
			ID:               uuid.MustParse("21c407b4-0f7b-4536-b3b3-2d40ed17b16d"),
			LEI:              "3FF600JQ0L6ANTGQ5G53",
			Domain:           "acme.com",
			Name:             "Acme Company, LTD",
			Website:          "https://acme.com",
			Country:          "CA",
			BusinessCategory: "BUSINESS_ENTITY",
			VASPCategories: []string{
				"Exchange",
				"DEX",
				"P2P",
				"Kiosk",
				"Custodian",
			},
			IVMS101:    json.RawMessage(`{"name":{"nameIdentifier":[{"legalPersonName":"Acme Company, LTD","legalPersonNameIdentifierType":"LEGL"}]},"countryOfRegistration":"CA"}`),
			VerifiedOn: time.Date(2024, 8, 2, 14, 5, 12, 0, time.UTC),
			Created:    time.Date(2023, 4, 17, 9, 23, 41, 0, time.UTC),
			Modified:   time.Date(2025, 11, 29, 18, 47, 6, 0, time.UTC),
		},
		{
			ID:               uuid.MustParse("9c865355-07b7-4013-b4c1-1d986def39d5"),
			LEI:              "T9ZI00ZJU0ZW77LOSA15",
			Domain:           "globex.de",
			Name:             "Globex Corporation",
			Website:          "https://globex.de",
			Country:          "DE",
			BusinessCategory: "PRIVATE_ORGANIZATION",
			VASPCategories: []string{
				"OTC",
				"Fund",
				"Gambling",
			},
			IVMS101:    json.RawMessage(`{"name":{"nameIdentifier":[{"legalPersonName":"Globex Corporation","legalPersonNameIdentifierType":"LEGL"}]},"countryOfRegistration":"DE"}`),
			VerifiedOn: time.Date(2023, 11, 8, 10, 31, 55, 0, time.UTC),
			Created:    time.Date(2022, 6, 21, 16, 2, 9, 0, time.UTC),
			Modified:   time.Date(2024, 3, 14, 7, 58, 30, 0, time.UTC),
		},
		{
			ID:               uuid.MustParse("2b507068-495b-4237-af70-852180a91d99"),
			LEI:              "ZCGR00MEOKMMY0RUSQ29",
			Domain:           "initech.org",
			Name:             "Initech Foundation",
			Website:          "https://initech.org",
			Country:          "US",
			BusinessCategory: "NON_COMMERCIAL_ENTITY",
			VASPCategories: []string{
				"Project",
				"Miner",
			},
			IVMS101:    json.RawMessage(`{"name":{"nameIdentifier":[{"legalPersonName":"Initech Foundation","legalPersonNameIdentifierType":"LEGL"}]},"countryOfRegistration":"US"}`),
			VerifiedOn: time.Date(2025, 5, 19, 21, 14, 3, 0, time.UTC),
			Created:    time.Date(2025, 2, 3, 12, 40, 27, 0, time.UTC),
			Modified:   time.Date(2026, 7, 30, 4, 19, 48, 0, time.UTC),
		},
	}
}

// companyRequest returns a copy of the fixture with the read-only fields zeroed,
// as a client would send it in a create or update request.
func companyRequest(fixture *api.Company) *api.Company {
	in := *fixture
	in.ID = uuid.Nil
	in.VerifiedOn = time.Time{}
	in.Created = time.Time{}
	in.Modified = time.Time{}
	return &in
}

func TestClient_ListCompanies(t *testing.T) {
	fixture := companyFixtures()

	t.Run("Ok", func(t *testing.T) {
		client := Serve(t, http.MethodGet, "/v1/companies", func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusOK, fixture)
			return nil
		})

		reply, err := client.ListCompanies(context.Background())
		require.NoError(t, err)
		require.Equal(t, fixture, reply)
	})

	t.Run("Empty", func(t *testing.T) {
		client := Serve(t, http.MethodGet, "/v1/companies", func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusOK, []*api.Company{})
			return nil
		})

		reply, err := client.ListCompanies(context.Background())
		require.NoError(t, err)
		require.Equal(t, []*api.Company{}, reply)
	})

	t.Run("Error", func(t *testing.T) {
		client := Serve(t, http.MethodGet, "/v1/companies", func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusInternalServerError, &api.Reply{Error: "Internal Server Error"})
			return nil
		})

		reply, err := client.ListCompanies(context.Background())
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusInternalServerError, err.(*api.StatusError).Code)
		require.Equal(t, "Internal Server Error", err.(*api.StatusError).Reply.Error)
	})
}

func TestClient_CreateCompany(t *testing.T) {
	fixture := companyFixtures()[0]

	t.Run("Ok", func(t *testing.T) {
		var received *api.Company
		client := Serve(t, http.MethodPost, "/v1/companies", func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			received = &api.Company{}
			if err := json.NewDecoder(r.Body).Decode(received); err != nil {
				return api.Error(http.StatusBadRequest, err, "")
			}
			WriteJSON(w, http.StatusCreated, fixture)
			return nil
		})

		in := companyRequest(fixture)
		reply, err := client.CreateCompany(context.Background(), in)
		require.NoError(t, err)
		require.Equal(t, in, received, "request body should match the input")
		require.Equal(t, fixture, reply)
	})

	t.Run("ValidationError", func(t *testing.T) {
		client := Serve(t, http.MethodPost, "/v1/companies", func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusBadRequest, &api.Reply{Error: "missing domain: this field is required"})
			return nil
		})

		in := companyRequest(fixture)
		in.Domain = ""
		reply, err := client.CreateCompany(context.Background(), in)
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusBadRequest, err.(*api.StatusError).Code)
		require.Equal(t, "missing domain: this field is required", err.(*api.StatusError).Reply.Error)
	})

	t.Run("Error", func(t *testing.T) {
		client := Serve(t, http.MethodPost, "/v1/companies", func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusInternalServerError, &api.Reply{Error: "Internal Server Error"})
			return nil
		})

		reply, err := client.CreateCompany(context.Background(), companyRequest(fixture))
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusInternalServerError, err.(*api.StatusError).Code)
		require.Equal(t, "Internal Server Error", err.(*api.StatusError).Reply.Error)
	})
}

func TestClient_RetrieveCompany(t *testing.T) {
	fixture := companyFixtures()[1]
	path := "/v1/companies/" + fixture.ID.String()

	t.Run("Ok", func(t *testing.T) {
		client := Serve(t, http.MethodGet, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusOK, fixture)
			return nil
		})

		reply, err := client.RetrieveCompany(context.Background(), fixture.ID)
		require.NoError(t, err)
		require.Equal(t, fixture, reply)
	})

	t.Run("NotFound", func(t *testing.T) {
		client := Serve(t, http.MethodGet, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusNotFound, &api.NotFound)
			return nil
		})

		reply, err := client.RetrieveCompany(context.Background(), fixture.ID)
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusNotFound, err.(*api.StatusError).Code)
		require.Equal(t, api.NotFound.Error, err.(*api.StatusError).Reply.Error)
	})

	t.Run("Error", func(t *testing.T) {
		client := Serve(t, http.MethodGet, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusInternalServerError, &api.Reply{Error: "Internal Server Error"})
			return nil
		})

		reply, err := client.RetrieveCompany(context.Background(), fixture.ID)
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusInternalServerError, err.(*api.StatusError).Code)
		require.Equal(t, "Internal Server Error", err.(*api.StatusError).Reply.Error)
	})
}

func TestClient_UpdateCompany(t *testing.T) {
	fixture := companyFixtures()[2]
	path := "/v1/companies/" + fixture.ID.String()

	t.Run("Ok", func(t *testing.T) {
		var received *api.Company
		client := Serve(t, http.MethodPut, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			received = &api.Company{}
			if err := json.NewDecoder(r.Body).Decode(received); err != nil {
				return api.Error(http.StatusBadRequest, err, "")
			}
			WriteJSON(w, http.StatusOK, fixture)
			return nil
		})

		// The ID is required on the input so the client can resolve the endpoint.
		in := companyRequest(fixture)
		in.ID = fixture.ID
		in.Website = "https://www.initech.org"

		reply, err := client.UpdateCompany(context.Background(), in)
		require.NoError(t, err)
		require.Equal(t, in, received, "request body should match the input")
		require.Equal(t, fixture, reply)
	})

	t.Run("ValidationError", func(t *testing.T) {
		detail := api.ErrorDetail{
			{Field: "name", Error: "missing name: this field is required"},
			{Field: "country", Error: "invalid country: invalid ISO 3166-1 alpha-2 code"},
		}

		client := Serve(t, http.MethodPut, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusBadRequest, &api.Reply{Error: "2 validation errors occurred", ErrorDetail: detail})
			return nil
		})

		in := companyRequest(fixture)
		in.ID = fixture.ID
		in.Name = ""
		in.Country = "USA"

		reply, err := client.UpdateCompany(context.Background(), in)
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusBadRequest, err.(*api.StatusError).Code)
		require.Equal(t, "2 validation errors occurred", err.(*api.StatusError).Reply.Error)
		require.Equal(t, detail, err.(*api.StatusError).Reply.ErrorDetail)
	})

	t.Run("NotFound", func(t *testing.T) {
		client := Serve(t, http.MethodPut, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusNotFound, &api.NotFound)
			return nil
		})

		in := companyRequest(fixture)
		in.ID = fixture.ID

		reply, err := client.UpdateCompany(context.Background(), in)
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusNotFound, err.(*api.StatusError).Code)
		require.Equal(t, api.NotFound.Error, err.(*api.StatusError).Reply.Error)
	})

	t.Run("Error", func(t *testing.T) {
		client := Serve(t, http.MethodPut, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusInternalServerError, &api.Reply{Error: "Internal Server Error"})
			return nil
		})

		in := companyRequest(fixture)
		in.ID = fixture.ID

		reply, err := client.UpdateCompany(context.Background(), in)
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusInternalServerError, err.(*api.StatusError).Code)
		require.Equal(t, "Internal Server Error", err.(*api.StatusError).Reply.Error)
	})
}

func TestClient_DeleteCompany(t *testing.T) {
	fixture := companyFixtures()[0]
	path := "/v1/companies/" + fixture.ID.String()

	t.Run("Ok", func(t *testing.T) {
		client := Serve(t, http.MethodDelete, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			w.WriteHeader(http.StatusNoContent)
			return nil
		})

		err := client.DeleteCompany(context.Background(), fixture.ID)
		require.NoError(t, err)
	})

	t.Run("NotFound", func(t *testing.T) {
		client := Serve(t, http.MethodDelete, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusNotFound, &api.NotFound)
			return nil
		})

		err := client.DeleteCompany(context.Background(), fixture.ID)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusNotFound, err.(*api.StatusError).Code)
		require.Equal(t, api.NotFound.Error, err.(*api.StatusError).Reply.Error)
	})

	t.Run("Error", func(t *testing.T) {
		client := Serve(t, http.MethodDelete, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusInternalServerError, &api.Reply{Error: "Internal Server Error"})
			return nil
		})

		err := client.DeleteCompany(context.Background(), fixture.ID)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusInternalServerError, err.(*api.StatusError).Code)
		require.Equal(t, "Internal Server Error", err.(*api.StatusError).Reply.Error)
	})
}

//============================================================================
// Test Contacts
//============================================================================

// contactFixtures returns a fresh copy of the contacts used across the client
// tests so that individual tests can safely modify them.
func contactFixtures() []*api.Contact {
	return []*api.Contact{
		{
			ID:        uuid.MustParse("135a608a-5c3b-4339-b40b-0a98e574a883"),
			Name:      "Franklin Wolf",
			Email:     "franklin.wolf@acme.com",
			Role:      "Compliance Officer",
			IVMS101:   json.RawMessage(`{"name":{"nameIdentifier":[{"primaryIdentifier":"Wolf","secondaryIdentifier":"Franklin","nameIdentifierType":"LEGL"}]}}`),
			CompanyID: uuid.MustParse("21c407b4-0f7b-4536-b3b3-2d40ed17b16d"),
			Created:   time.Date(2024, 9, 14, 11, 7, 33, 0, time.UTC),
			Modified:  time.Date(2025, 3, 22, 16, 48, 19, 0, time.UTC),
		},
		{
			ID:        uuid.MustParse("9f572069-92ea-4fed-9a43-34a61652f679"),
			Name:      "Jessica Carpenter",
			Email:     "jessica.carpenter@acme.com",
			Role:      "Legal Counsel",
			IVMS101:   json.RawMessage(`{"name":{"nameIdentifier":[{"primaryIdentifier":"Carpenter","secondaryIdentifier":"Jessica","nameIdentifierType":"LEGL"}]}}`),
			CompanyID: uuid.MustParse("21c407b4-0f7b-4536-b3b3-2d40ed17b16d"),
			Created:   time.Date(2025, 1, 27, 8, 52, 4, 0, time.UTC),
			Modified:  time.Date(2025, 10, 5, 13, 26, 57, 0, time.UTC),
		},
		{
			ID:        uuid.MustParse("4d2f8c1e-6b7a-4e35-a9d0-73c5e2f1b864"),
			Name:      "Hans Richter",
			Email:     "hans.richter@globex.de",
			Role:      "Compliance Officer",
			IVMS101:   json.RawMessage(`{"name":{"nameIdentifier":[{"primaryIdentifier":"Richter","secondaryIdentifier":"Hans","nameIdentifierType":"LEGL"}]}}`),
			CompanyID: uuid.MustParse("9c865355-07b7-4013-b4c1-1d986def39d5"),
			Created:   time.Date(2023, 12, 5, 9, 41, 17, 0, time.UTC),
			Modified:  time.Date(2024, 6, 18, 15, 3, 52, 0, time.UTC),
		},
		{
			ID:        uuid.MustParse("b81e3a57-0c94-4f2d-8e6b-5a7d1c9f0e23"),
			Name:      "Anneliese Vogel",
			Email:     "anneliese.vogel@globex.de",
			Role:      "Technical Contact",
			IVMS101:   json.RawMessage(`{"name":{"nameIdentifier":[{"primaryIdentifier":"Vogel","secondaryIdentifier":"Anneliese","nameIdentifierType":"LEGL"}]}}`),
			CompanyID: uuid.MustParse("9c865355-07b7-4013-b4c1-1d986def39d5"),
			Created:   time.Date(2024, 2, 9, 20, 16, 44, 0, time.UTC),
			Modified:  time.Date(2025, 1, 23, 6, 37, 28, 0, time.UTC),
		},
		{
			ID:        uuid.MustParse("e6a09d3b-2f18-47c6-b5d4-91f0a7c3e852"),
			Name:      "Priya Natarajan",
			Email:     "priya.natarajan@initech.org",
			Role:      "Executive Director",
			IVMS101:   json.RawMessage(`{"name":{"nameIdentifier":[{"primaryIdentifier":"Natarajan","secondaryIdentifier":"Priya","nameIdentifierType":"LEGL"}]}}`),
			CompanyID: uuid.MustParse("2b507068-495b-4237-af70-852180a91d99"),
			Created:   time.Date(2025, 9, 8, 17, 29, 6, 0, time.UTC),
			Modified:  time.Date(2026, 4, 11, 10, 54, 39, 0, time.UTC),
		},
	}
}

func contactRequest(fixture *api.Contact) *api.Contact {
	in := *fixture
	in.ID = uuid.Nil
	in.Created = time.Time{}
	in.Modified = time.Time{}
	return &in
}

func TestClient_ListCompanyContacts(t *testing.T) {
	// Acme has two contacts in the fixtures.
	company := companyFixtures()[0]
	fixture := contactFixtures()[0:2]
	path := "/v1/companies/" + company.ID.String() + "/contacts"

	t.Run("Ok", func(t *testing.T) {
		client := Serve(t, http.MethodGet, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusOK, fixture)
			return nil
		})

		reply, err := client.ListCompanyContacts(context.Background(), company.ID)
		require.NoError(t, err)
		require.Equal(t, fixture, reply)
		for _, contact := range reply {
			require.Equal(t, company.ID, contact.CompanyID, "expected all contacts to belong to the company")
		}
	})

	t.Run("Empty", func(t *testing.T) {
		client := Serve(t, http.MethodGet, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusOK, []*api.Contact{})
			return nil
		})

		reply, err := client.ListCompanyContacts(context.Background(), company.ID)
		require.NoError(t, err)
		require.Equal(t, []*api.Contact{}, reply)
	})

	t.Run("NotFound", func(t *testing.T) {
		client := Serve(t, http.MethodGet, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusNotFound, &api.NotFound)
			return nil
		})

		reply, err := client.ListCompanyContacts(context.Background(), company.ID)
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusNotFound, err.(*api.StatusError).Code)
		require.Equal(t, api.NotFound.Error, err.(*api.StatusError).Reply.Error)
	})

	t.Run("Error", func(t *testing.T) {
		client := Serve(t, http.MethodGet, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusInternalServerError, &api.Reply{Error: "Internal Server Error"})
			return nil
		})

		reply, err := client.ListCompanyContacts(context.Background(), company.ID)
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusInternalServerError, err.(*api.StatusError).Code)
		require.Equal(t, "Internal Server Error", err.(*api.StatusError).Reply.Error)
	})
}

func TestClient_CreateCompanyContact(t *testing.T) {
	// Hans Richter belongs to Globex.
	company := companyFixtures()[1]
	fixture := contactFixtures()[2]
	path := "/v1/companies/" + company.ID.String() + "/contacts"

	t.Run("Ok", func(t *testing.T) {
		var received *api.Contact
		client := Serve(t, http.MethodPost, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			received = &api.Contact{}
			if err := json.NewDecoder(r.Body).Decode(received); err != nil {
				return api.Error(http.StatusBadRequest, err, "")
			}
			WriteJSON(w, http.StatusCreated, fixture)
			return nil
		})

		in := contactRequest(fixture)
		reply, err := client.CreateCompanyContact(context.Background(), in)
		require.NoError(t, err)
		require.Equal(t, in, received, "request body should match the input")
		require.Equal(t, fixture, reply)
	})

	t.Run("MissingCompanyID", func(t *testing.T) {
		// The client refuses to send the request without a company ID to build the endpoint.
		var called bool
		client := Serve(t, http.MethodPost, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			called = true
			WriteJSON(w, http.StatusCreated, fixture)
			return nil
		})

		in := contactRequest(fixture)
		in.CompanyID = uuid.Nil
		reply, err := client.CreateCompanyContact(context.Background(), in)
		require.Nil(t, reply)
		require.EqualError(t, err, "company ID is required")
		require.False(t, called, "expected no request to be sent to the server")
	})

	t.Run("ValidationError", func(t *testing.T) {
		client := Serve(t, http.MethodPost, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusBadRequest, &api.Reply{Error: "missing email: this field is required"})
			return nil
		})

		in := contactRequest(fixture)
		in.Email = ""
		reply, err := client.CreateCompanyContact(context.Background(), in)
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusBadRequest, err.(*api.StatusError).Code)
		require.Equal(t, "missing email: this field is required", err.(*api.StatusError).Reply.Error)
	})

	t.Run("NotFound", func(t *testing.T) {
		client := Serve(t, http.MethodPost, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusNotFound, &api.NotFound)
			return nil
		})

		reply, err := client.CreateCompanyContact(context.Background(), contactRequest(fixture))
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusNotFound, err.(*api.StatusError).Code)
		require.Equal(t, api.NotFound.Error, err.(*api.StatusError).Reply.Error)
	})

	t.Run("Error", func(t *testing.T) {
		client := Serve(t, http.MethodPost, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusInternalServerError, &api.Reply{Error: "Internal Server Error"})
			return nil
		})

		reply, err := client.CreateCompanyContact(context.Background(), contactRequest(fixture))
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusInternalServerError, err.(*api.StatusError).Code)
		require.Equal(t, "Internal Server Error", err.(*api.StatusError).Reply.Error)
	})
}

func TestClient_ListContacts(t *testing.T) {
	fixture := contactFixtures()

	t.Run("Ok", func(t *testing.T) {
		client := Serve(t, http.MethodGet, "/v1/contacts", func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusOK, fixture)
			return nil
		})

		reply, err := client.ListContacts(context.Background())
		require.NoError(t, err)
		require.Equal(t, fixture, reply)
	})

	t.Run("Empty", func(t *testing.T) {
		client := Serve(t, http.MethodGet, "/v1/contacts", func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusOK, []*api.Contact{})
			return nil
		})

		reply, err := client.ListContacts(context.Background())
		require.NoError(t, err)
		require.Equal(t, []*api.Contact{}, reply)
	})

	t.Run("Error", func(t *testing.T) {
		client := Serve(t, http.MethodGet, "/v1/contacts", func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusInternalServerError, &api.Reply{Error: "Internal Server Error"})
			return nil
		})

		reply, err := client.ListContacts(context.Background())
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusInternalServerError, err.(*api.StatusError).Code)
		require.Equal(t, "Internal Server Error", err.(*api.StatusError).Reply.Error)
	})
}

func TestClient_CreateContact(t *testing.T) {
	// Jessica Carpenter belongs to Acme.
	fixture := contactFixtures()[1]

	t.Run("Ok", func(t *testing.T) {
		var received *api.Contact
		client := Serve(t, http.MethodPost, "/v1/contacts", func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			received = &api.Contact{}
			if err := json.NewDecoder(r.Body).Decode(received); err != nil {
				return api.Error(http.StatusBadRequest, err, "")
			}
			WriteJSON(w, http.StatusCreated, fixture)
			return nil
		})

		in := contactRequest(fixture)
		reply, err := client.CreateContact(context.Background(), in)
		require.NoError(t, err)
		require.Equal(t, in, received, "request body should match the input")
		require.Equal(t, fixture, reply)
	})

	t.Run("NoCompanyID", func(t *testing.T) {
		// Unlike CreateCompanyContact, a contact may be created without a company.
		var received *api.Contact
		client := Serve(t, http.MethodPost, "/v1/contacts", func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			received = &api.Contact{}
			if err := json.NewDecoder(r.Body).Decode(received); err != nil {
				return api.Error(http.StatusBadRequest, err, "")
			}
			WriteJSON(w, http.StatusCreated, fixture)
			return nil
		})

		in := contactRequest(fixture)
		in.CompanyID = uuid.Nil
		reply, err := client.CreateContact(context.Background(), in)
		require.NoError(t, err)
		require.Equal(t, in, received, "request body should match the input")
		require.Equal(t, uuid.Nil, received.CompanyID, "expected company_id to be omitted from the request")
		require.Equal(t, fixture, reply)
	})

	t.Run("ValidationError", func(t *testing.T) {
		client := Serve(t, http.MethodPost, "/v1/contacts", func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusBadRequest, &api.Reply{Error: "missing name: this field is required"})
			return nil
		})

		in := contactRequest(fixture)
		in.Name = ""
		reply, err := client.CreateContact(context.Background(), in)
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusBadRequest, err.(*api.StatusError).Code)
		require.Equal(t, "missing name: this field is required", err.(*api.StatusError).Reply.Error)
	})

	t.Run("Error", func(t *testing.T) {
		client := Serve(t, http.MethodPost, "/v1/contacts", func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusInternalServerError, &api.Reply{Error: "Internal Server Error"})
			return nil
		})

		reply, err := client.CreateContact(context.Background(), contactRequest(fixture))
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusInternalServerError, err.(*api.StatusError).Code)
		require.Equal(t, "Internal Server Error", err.(*api.StatusError).Reply.Error)
	})
}

func TestClient_RetrieveContact(t *testing.T) {
	// Anneliese Vogel belongs to Globex.
	fixture := contactFixtures()[3]
	path := "/v1/contacts/" + fixture.ID.String()

	t.Run("Ok", func(t *testing.T) {
		client := Serve(t, http.MethodGet, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusOK, fixture)
			return nil
		})

		reply, err := client.RetrieveContact(context.Background(), fixture.ID)
		require.NoError(t, err)
		require.Equal(t, fixture, reply)
	})

	t.Run("NotFound", func(t *testing.T) {
		client := Serve(t, http.MethodGet, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusNotFound, &api.NotFound)
			return nil
		})

		reply, err := client.RetrieveContact(context.Background(), fixture.ID)
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusNotFound, err.(*api.StatusError).Code)
		require.Equal(t, api.NotFound.Error, err.(*api.StatusError).Reply.Error)
	})

	t.Run("Error", func(t *testing.T) {
		client := Serve(t, http.MethodGet, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusInternalServerError, &api.Reply{Error: "Internal Server Error"})
			return nil
		})

		reply, err := client.RetrieveContact(context.Background(), fixture.ID)
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusInternalServerError, err.(*api.StatusError).Code)
		require.Equal(t, "Internal Server Error", err.(*api.StatusError).Reply.Error)
	})
}

func TestClient_UpdateContact(t *testing.T) {
	// Priya Natarajan belongs to Initech.
	fixture := contactFixtures()[4]
	path := "/v1/contacts/" + fixture.ID.String()

	t.Run("Ok", func(t *testing.T) {
		var received *api.Contact
		client := Serve(t, http.MethodPut, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			received = &api.Contact{}
			if err := json.NewDecoder(r.Body).Decode(received); err != nil {
				return api.Error(http.StatusBadRequest, err, "")
			}
			WriteJSON(w, http.StatusOK, fixture)
			return nil
		})

		// The ID is required on the input so the client can resolve the endpoint.
		in := contactRequest(fixture)
		in.ID = fixture.ID
		in.Role = "Managing Director"

		reply, err := client.UpdateContact(context.Background(), in)
		require.NoError(t, err)
		require.Equal(t, in, received, "request body should match the input")
		require.Equal(t, fixture, reply)
	})

	t.Run("ValidationError", func(t *testing.T) {
		detail := api.ErrorDetail{
			{Field: "name", Error: "missing name: this field is required"},
			{Field: "email", Error: "invalid email: mail: missing '@' or angle-addr"},
		}

		client := Serve(t, http.MethodPut, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusBadRequest, &api.Reply{Error: "2 validation errors occurred", ErrorDetail: detail})
			return nil
		})

		in := contactRequest(fixture)
		in.ID = fixture.ID
		in.Name = ""
		in.Email = "not-an-email"

		reply, err := client.UpdateContact(context.Background(), in)
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusBadRequest, err.(*api.StatusError).Code)
		require.Equal(t, "2 validation errors occurred", err.(*api.StatusError).Reply.Error)
		require.Equal(t, detail, err.(*api.StatusError).Reply.ErrorDetail)
	})

	t.Run("NotFound", func(t *testing.T) {
		client := Serve(t, http.MethodPut, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusNotFound, &api.NotFound)
			return nil
		})

		in := contactRequest(fixture)
		in.ID = fixture.ID

		reply, err := client.UpdateContact(context.Background(), in)
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusNotFound, err.(*api.StatusError).Code)
		require.Equal(t, api.NotFound.Error, err.(*api.StatusError).Reply.Error)
	})

	t.Run("Error", func(t *testing.T) {
		client := Serve(t, http.MethodPut, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusInternalServerError, &api.Reply{Error: "Internal Server Error"})
			return nil
		})

		in := contactRequest(fixture)
		in.ID = fixture.ID

		reply, err := client.UpdateContact(context.Background(), in)
		require.Nil(t, reply)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusInternalServerError, err.(*api.StatusError).Code)
		require.Equal(t, "Internal Server Error", err.(*api.StatusError).Reply.Error)
	})
}

func TestClient_DeleteContact(t *testing.T) {
	// Franklin Wolf belongs to Acme.
	fixture := contactFixtures()[0]
	path := "/v1/contacts/" + fixture.ID.String()

	t.Run("Ok", func(t *testing.T) {
		client := Serve(t, http.MethodDelete, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			w.WriteHeader(http.StatusNoContent)
			return nil
		})

		err := client.DeleteContact(context.Background(), fixture.ID)
		require.NoError(t, err)
	})

	t.Run("NotFound", func(t *testing.T) {
		client := Serve(t, http.MethodDelete, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusNotFound, &api.NotFound)
			return nil
		})

		err := client.DeleteContact(context.Background(), fixture.ID)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusNotFound, err.(*api.StatusError).Code)
		require.Equal(t, api.NotFound.Error, err.(*api.StatusError).Reply.Error)
	})

	t.Run("Error", func(t *testing.T) {
		client := Serve(t, http.MethodDelete, path, func(w http.ResponseWriter, r *http.Request) *errors.HTTP {
			WriteJSON(w, http.StatusInternalServerError, &api.Reply{Error: "Internal Server Error"})
			return nil
		})

		err := client.DeleteContact(context.Background(), fixture.ID)
		require.Error(t, err)
		require.IsType(t, &api.StatusError{}, err)
		require.Equal(t, http.StatusInternalServerError, err.(*api.StatusError).Code)
		require.Equal(t, "Internal Server Error", err.(*api.StatusError).Reply.Error)
	})
}

//============================================================================
// Client Test Helper
//============================================================================

func WriteJSON(w http.ResponseWriter, status int, reply any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(reply)
}

func Serve(t *testing.T, method, path string, handler Handler) (client api.Client) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != path {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		if err := handler(w, r); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(err.Status)
			json.NewEncoder(w).Encode(err.Reply)
			return
		}
	}))

	t.Cleanup(server.Close)

	client, _ = api.New(server.URL, api.WithClient(server.Client()))
	return client
}

type Handler func(w http.ResponseWriter, r *http.Request) *errors.HTTP
