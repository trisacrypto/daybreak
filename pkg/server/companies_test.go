package server

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/trisacrypto/daybreak/pkg/api/v1"
)

// cSpell:ignore globex
func (s *Suite) TestCompanyResource() {
	s.T().Cleanup(s.ResetDB)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.Run("ListCompanies", func() {
		require := s.Require()
		companies, err := s.client.ListCompanies(ctx)
		require.NoError(err, "could not list companies")
		require.Equal(3, len(companies), "should have 3 companies")
	})

	s.Run("CreateCompany", func() {
		s.Run("Success", func() {
			require := s.Require()
			company := &api.Company{
				LEI:              "GSAT00N4Q1AKFAMMDK61",
				Domain:           "etoile-numerique.fr",
				Name:             "Étoile Numérique",
				Website:          "https://etoile-numerique.fr",
				Country:          "FR",
				BusinessCategory: "PRIVATE_ORGANIZATION",
				VASPCategories:   []string{"Custodian", "Project"},
				IVMS101:          json.RawMessage([]byte("null")),
			}

			created, err := s.client.CreateCompany(ctx, company)
			require.NoError(err, "could not create company")

			require.Equal(created.Clone(), company, "created company should be the same as the input without read-only fields")
			require.NotZero(created.ID, "id should be set")
			require.NotZero(created.Created, "created should be set")
			require.NotZero(created.Modified, "modified should be set")
			require.Zero(created.VerifiedOn, "verified on should be zero")
		})

		s.Run("Empty", func() {
			require := s.Require()
			created, err := s.client.CreateCompany(ctx, &api.Company{})
			require.Error(err, "expected error creating company with empty company")
			require.EqualError(err, "[422] 5 validation errors occurred")
			require.Nil(created, "company should be nil")
		})

		s.Run("DomainConflict", func() {
			require := s.Require()
			company := &api.Company{
				LEI:              "GSAT00N4Q1AKFAMMDK61",
				Domain:           "acme.com",
				Name:             "Étoile Numérique",
				Website:          "https://etoile-numerique.fr",
				Country:          "FR",
				BusinessCategory: "PRIVATE_ORGANIZATION",
				VASPCategories:   []string{"Custodian", "Project"},
				IVMS101:          json.RawMessage([]byte("null")),
			}

			created, err := s.client.CreateCompany(ctx, company)
			require.Error(err, "expected error creating company with domain conflict")
			require.EqualError(err, "[409] company already exists")
			require.Nil(created, "company should be nil")
		})
	})

	s.Run("RetrieveCompany", func() {
		s.Run("Found", func() {
			require := s.Require()
			company, err := s.client.RetrieveCompany(ctx, uuid.MustParse("21c407b4-0f7b-4536-b3b3-2d40ed17b16d"))
			require.NoError(err, "could not retrieve company")
			require.Equal("Acme Company, LTD", company.Name, "name should be Acme Company, LTD")
		})

		s.Run("NotFound", func() {
			require := s.Require()
			company, err := s.client.RetrieveCompany(ctx, uuid.MustParse("ae828dd0-5d13-401a-b336-65e6f3d91e31"))
			require.Nil(company, "company should be nil")
			require.Error(err, "could not retrieve company")
			require.EqualError(err, "[404] company not found")
		})
	})

	s.Run("UpdateCompany", func() {
		original, err := s.client.RetrieveCompany(ctx, uuid.MustParse("21c407b4-0f7b-4536-b3b3-2d40ed17b16d"))
		s.Require().NoError(err, "could not retrieve company")

		s.Run("Success", func() {
			require := s.Require()
			original.Name = "Acme Company, Inc."
			updated, err := s.client.UpdateCompany(ctx, original)
			require.NoError(err, "could not update company")
			require.Equal("Acme Company, Inc.", updated.Name, "name should be Acme Company, Inc.")
			require.Equal(original.ID, updated.ID, "id should be the same")
			require.Equal(original.LEI, updated.LEI, "lei should be the same")
			require.Equal(original.Domain, updated.Domain, "domain should be the same")
			require.Equal(original.Name, updated.Name, "name should be the same")
			require.Equal(original.Website, updated.Website, "website should be the same")
			require.Equal(original.Country, updated.Country, "country should be the same")
			require.Equal(original.BusinessCategory, updated.BusinessCategory, "business category should be the same")
			require.Equal(original.VASPCategories, updated.VASPCategories, "vasp categories should be the same")
			require.Equal(original.IVMS101, updated.IVMS101, "ivms101 should be the same")
			require.Equal(original.VerifiedOn, updated.VerifiedOn, "verified on should be the same")
			require.Equal(original.Created, updated.Created, "created should be the same")
			require.NotEqual(original.Modified, updated.Modified, "modified should be different")

			updated, err = s.client.RetrieveCompany(ctx, original.ID)
			require.NoError(err, "could not retrieve company")
			require.Equal("Acme Company, Inc.", updated.Name, "name should be Acme Company, Inc.")
			require.NotEqual(original.Modified, updated.Modified, "modified should be different")
		})

		s.Run("NotFound", func() {
			require := s.Require()
			company := &api.Company{
				ID:               uuid.MustParse("a40444b8-9237-404a-b336-5a6c1461c892"),
				LEI:              original.LEI,
				Domain:           original.Domain,
				Name:             original.Name,
				Website:          original.Website,
				Country:          original.Country,
				BusinessCategory: original.BusinessCategory,
				VASPCategories:   original.VASPCategories,
				IVMS101:          original.IVMS101,
			}

			updated, err := s.client.UpdateCompany(ctx, company)
			require.Error(err, "could not update company")
			require.EqualError(err, "[404] company not found")
			require.Nil(updated, "company should be nil")

		})

		s.Run("DomainConflict", func() {
			require := s.Require()
			company := &api.Company{
				ID:               original.ID,
				LEI:              original.LEI,
				Domain:           "globex.de",
				Name:             original.Name,
				Website:          original.Website,
				Country:          original.Country,
				BusinessCategory: original.BusinessCategory,
				VASPCategories:   original.VASPCategories,
				IVMS101:          original.IVMS101,
			}

			updated, err := s.client.UpdateCompany(ctx, company)
			require.Error(err, "could not update company")
			require.EqualError(err, "[409] company already exists")
			require.Nil(updated, "company should be nil")
		})
	})

	s.Run("DeleteCompany", func() {
		s.Run("Success", func() {
			require := s.Require()
			err := s.client.DeleteCompany(ctx, uuid.MustParse("21c407b4-0f7b-4536-b3b3-2d40ed17b16d"))
			require.NoError(err, "could not delete company")

			company, err := s.client.RetrieveCompany(ctx, uuid.MustParse("21c407b4-0f7b-4536-b3b3-2d40ed17b16d"))
			require.Nil(company, "company should be nil")
			require.Error(err, "could not retrieve company")
			require.EqualError(err, "[404] company not found")
		})

		s.Run("NotFound", func() {
			require := s.Require()
			err := s.client.DeleteCompany(ctx, uuid.MustParse("ae828dd0-5d13-401a-b336-65e6f3d91e31"))
			require.Error(err, "could not delete company")
			require.EqualError(err, "[404] company not found")
		})
	})
}
