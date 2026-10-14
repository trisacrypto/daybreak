package server

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/trisacrypto/daybreak/pkg/api/v1"
)

// cSpell:ignore Relka
func (s *Suite) TestContactResource() {
	s.T().Cleanup(s.ResetDB)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.Run("ListContacts", func() {
		require := s.Require()
		contacts, err := s.client.ListContacts(ctx)
		require.NoError(err, "could not list contacts")
		require.Equal(5, len(contacts), "should have 5 contacts")
	})

	s.Run("ListCompanyContacts", func() {
		require := s.Require()
		contacts, err := s.client.ListCompanyContacts(ctx, uuid.MustParse("21c407b4-0f7b-4536-b3b3-2d40ed17b16d"))
		require.NoError(err, "could not list company contacts")
		require.Equal(2, len(contacts), "should have 2 company contacts")
	})

	s.Run("CreateContact", func() {
		defer s.ResetDB()

		s.Run("Success", func() {
			require := s.Require()
			contact := &api.Contact{
				Name:      "Victoria Relka",
				Email:     "victoria.relka@globex.de",
				Role:      "Compliance Intern",
				CompanyID: uuid.MustParse("9c865355-07b7-4013-b4c1-1d986def39d5"),
				IVMS101:   json.RawMessage([]byte("null")),
			}

			created, err := s.client.CreateContact(ctx, contact)
			require.NoError(err, "could not create contact")
			require.Equal(created.Clone(), contact, "created contact should be the same as the input without read-only fields")
			require.NotZero(created.ID, "id should be set")
			require.NotZero(created.Created, "created should be set")
			require.NotZero(created.Modified, "modified should be set")
		})

		s.Run("Empty", func() {
			require := s.Require()
			created, err := s.client.CreateContact(ctx, &api.Contact{})
			require.Error(err, "expected error creating contact with empty contact")
			require.EqualError(err, "[422] 2 validation errors occurred")
			require.Nil(created, "contact should be nil")
		})

		s.Run("CompanyIDRequired", func() {
			require := s.Require()
			created, err := s.client.CreateContact(ctx, &api.Contact{
				Name:  "Victoria Relka",
				Email: "victoria.relka@globex.de",
				Role:  "Compliance Intern",
			})
			require.Error(err, "expected error creating contact with company ID required")
			require.EqualError(err, "[422] company ID is required")
			require.Nil(created, "contact should be nil")
		})

		s.Run("CompanyNotFound", func() {
			require := s.Require()
			contact := &api.Contact{
				Name:      "Victoria Relka",
				Email:     "victoria.relka@globex.de",
				Role:      "Compliance Intern",
				CompanyID: uuid.MustParse("ae828dd0-5d13-401a-b336-65e6f3d91e31"),
			}

			created, err := s.client.CreateContact(ctx, contact)
			require.Error(err, "expected error creating contact with company not found")
			require.EqualError(err, "[404] company not found")
			require.Nil(created, "contact should be nil")
		})

		s.Run("ContactAlreadyExists", func() {
			require := s.Require()
			contact := &api.Contact{
				Name:      "Victoria Relka",
				Email:     "victoria.relka@globex.de",
				Role:      "Compliance Intern",
				CompanyID: uuid.MustParse("9c865355-07b7-4013-b4c1-1d986def39d5"),
			}

			created, err := s.client.CreateContact(ctx, contact)
			require.Error(err, "expected error creating contact with contact already exists")
			require.EqualError(err, "[409] contact already exists")
			require.Nil(created, "contact should be nil")
		})
	})

	s.Run("CreateCompanyContact", func() {
		defer s.ResetDB()

		s.Run("Success", func() {
			require := s.Require()
			contact := &api.Contact{
				Name:    "Victoria Relka",
				Email:   "victoria.relka@globex.de",
				Role:    "Compliance Intern",
				IVMS101: json.RawMessage([]byte("null")),
			}

			companyID := uuid.MustParse("9c865355-07b7-4013-b4c1-1d986def39d5")
			created, err := s.client.CreateCompanyContact(ctx, companyID, contact)
			require.NoError(err, "could not create contact")

			expected := contact.Clone()
			expected.CompanyID = companyID

			require.Equal(expected, created.Clone(), "created contact should be the same as the input without read-only fields")
			require.NotZero(created.ID, "id should be set")
			require.NotZero(created.Created, "created should be set")
			require.NotZero(created.Modified, "modified should be set")
		})

		s.Run("Empty", func() {
			require := s.Require()
			companyID := uuid.MustParse("9c865355-07b7-4013-b4c1-1d986def39d5")
			created, err := s.client.CreateCompanyContact(ctx, companyID, &api.Contact{})
			require.Error(err, "expected error creating contact with empty contact")
			require.EqualError(err, "[422] 2 validation errors occurred")
			require.Nil(created, "contact should be nil")
		})

		s.Run("CompanyIDRequired", func() {
			require := s.Require()
			created, err := s.client.CreateCompanyContact(ctx, uuid.Nil, &api.Contact{
				Name:  "Victoria Relka",
				Email: "victoria.relka@globex.de",
				Role:  "Compliance Intern",
			})
			require.Error(err, "expected error creating contact with company ID required")
			require.EqualError(err, "[400] invalid company ID")
			require.Nil(created, "contact should be nil")
		})

		s.Run("CompanyNotFound", func() {
			require := s.Require()
			contact := &api.Contact{
				Name:  "Victoria Relka",
				Email: "victoria.relka@globex.de",
				Role:  "Compliance Intern",
			}

			companyID := uuid.MustParse("ae828dd0-5d13-401a-b336-65e6f3d91e31")
			created, err := s.client.CreateCompanyContact(ctx, companyID, contact)
			require.Error(err, "expected error creating contact with company not found")
			require.EqualError(err, "[404] company not found")
			require.Nil(created, "contact should be nil")
		})

		s.Run("ContactAlreadyExists", func() {
			require := s.Require()
			contact := &api.Contact{
				Name:  "Victoria Relka",
				Email: "victoria.relka@globex.de",
				Role:  "Compliance Intern",
			}

			companyID := uuid.MustParse("9c865355-07b7-4013-b4c1-1d986def39d5")
			created, err := s.client.CreateCompanyContact(ctx, companyID, contact)
			require.Error(err, "expected error creating contact with contact already exists")
			require.EqualError(err, "[409] contact already exists")
			require.Nil(created, "contact should be nil")
		})
	})

	//cSpell:ignore Priya
	s.Run("RetrieveContact", func() {
		s.Run("Found", func() {
			require := s.Require()
			contactID := uuid.MustParse("e6a09d3b-2f18-47c6-b5d4-91f0a7c3e852")
			contact, err := s.client.RetrieveContact(ctx, contactID)
			require.NoError(err, "could not retrieve contact")
			require.Equal(contact.ID, contactID, "contact ID should match")
			require.Equal(contact.Name, "Priya Natarajan", "contact name should match")
			require.Equal(contact.Email, "priya.natarajan@initech.org", "contact email should match")
			require.Equal(contact.Role, "Executive Director", "contact role should match")
			require.Equal(contact.CompanyID, uuid.MustParse("2b507068-495b-4237-af70-852180a91d99"), "contact company ID should match")
			require.NotZero(contact.Created, "created should be set")
			require.NotZero(contact.Modified, "modified should be set")
		})

		s.Run("NotFound", func() {
			require := s.Require()
			contact, err := s.client.RetrieveContact(ctx, uuid.MustParse("d57a0bf9-5195-4986-8956-4c26b2c0d11b"))
			require.Error(err, "expected error retrieving contact not found")
			require.EqualError(err, "[404] contact not found")
			require.Nil(contact, "contact should be nil")
		})
	})

	s.Run("UpdateContact", func() {
		contactID := uuid.MustParse("e6a09d3b-2f18-47c6-b5d4-91f0a7c3e852")
		original, err := s.client.RetrieveContact(ctx, contactID)
		s.Require().NoError(err, "could not retrieve contact")

		s.Run("Success", func() {
			require := s.Require()
			original.Role = "Chief Counsel"
			updated, err := s.client.UpdateContact(ctx, original)
			s.Require().NoError(err, "could not update contact")
			require.Equal(original.ID, updated.ID, "id should be the same")
			require.Equal(original.Name, updated.Name, "name should be the same")
			require.Equal(original.Email, updated.Email, "email should be the same")
			require.Equal(original.CompanyID, updated.CompanyID, "company ID should be the same")
			require.Equal(original.IVMS101, updated.IVMS101, "ivms101 should be the same")
			require.Equal(original.Created, updated.Created, "created should be the same")
			require.NotEqual(original.Modified, updated.Modified, "modified should be different")

			updated, err = s.client.RetrieveContact(ctx, original.ID)
			s.Require().NoError(err, "could not retrieve contact")
			require.Equal(original.ID, updated.ID, "id should be the same")
			require.Equal("Chief Counsel", updated.Role, "name should be the same")
		})

		s.Run("NotFound", func() {
			require := s.Require()
			contact := &api.Contact{
				ID:        uuid.MustParse("8121a07f-7298-49cd-9975-727d02ca2ce7"),
				Name:      original.Name,
				Email:     original.Email,
				Role:      original.Role,
				IVMS101:   original.IVMS101,
				CompanyID: original.CompanyID,
				Created:   original.Created,
				Modified:  original.Modified,
			}

			updated, err := s.client.UpdateContact(ctx, contact)
			require.Error(err, "expected error updating contact not found")
			require.EqualError(err, "[404] contact not found")
			require.Nil(updated, "contact should be nil")
		})

		s.Run("ContactAlreadyExists", func() {
			require := s.Require()
			contact := &api.Contact{
				ID:        original.ID,
				Name:      original.Name,
				Email:     "franklin.wolf@acme.com",
				Role:      original.Role,
				IVMS101:   original.IVMS101,
				CompanyID: original.CompanyID,
				Created:   original.Created,
				Modified:  original.Modified,
			}

			updated, err := s.client.UpdateContact(ctx, contact)
			require.Error(err, "expected error updating contact contact already exists")
			require.EqualError(err, "[409] contact already exists")
			require.Nil(updated, "contact should be nil")
		})

		s.Run("CompanyNotFound", func() {
			require := s.Require()
			contact := &api.Contact{
				ID:        original.ID,
				Name:      original.Name,
				Email:     original.Email,
				Role:      original.Role,
				IVMS101:   original.IVMS101,
				CompanyID: uuid.MustParse("238706c8-a4a5-4134-b519-d1c7ae1a6044"),
				Created:   original.Created,
				Modified:  original.Modified,
			}

			updated, err := s.client.UpdateContact(ctx, contact)
			require.Error(err, "expected error updating contact contact already exists")
			require.EqualError(err, "[404] company not found")
			require.Nil(updated, "contact should be nil")
		})

	})

	s.Run("DeleteContact", func() {
		s.Run("Success", func() {
			require := s.Require()
			contactID := uuid.MustParse("e6a09d3b-2f18-47c6-b5d4-91f0a7c3e852")
			err := s.client.DeleteContact(ctx, contactID)
			require.NoError(err, "could not delete contact")

			contact, err := s.client.RetrieveContact(ctx, contactID)
			require.Error(err, "expected error retrieving contact not found")
			require.EqualError(err, "[404] contact not found")
			require.Nil(contact, "contact should be nil")
		})

		s.Run("NotFound", func() {
			require := s.Require()
			err := s.client.DeleteContact(ctx, uuid.MustParse("d57a0bf9-5195-4986-8956-4c26b2c0d11b"))
			require.Error(err, "expected error deleting contact not found")
			require.EqualError(err, "[404] contact not found")
		})
	})
}
