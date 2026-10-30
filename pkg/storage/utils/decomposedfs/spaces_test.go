// Copyright 2018-2022 CERN
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// In applying this license, CERN does not waive the privileges and immunities
// granted to it by virtue of its status as an Intergovernmental Organization
// or submit itself to any jurisdiction.

package decomposedfs_test

import (
	"context"

	userv1beta1 "github.com/cs3org/go-cs3apis/cs3/identity/user/v1beta1"
	cs3permissions "github.com/cs3org/go-cs3apis/cs3/permissions/v1beta1"
	rpcv1beta1 "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	typesv1beta1 "github.com/cs3org/go-cs3apis/cs3/types/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctxpkg "github.com/opencloud-eu/reva/v2/pkg/ctx"
	"github.com/opencloud-eu/reva/v2/pkg/errtypes"
	"github.com/opencloud-eu/reva/v2/pkg/storage/utils/decomposedfs/node"
	helpers "github.com/opencloud-eu/reva/v2/pkg/storage/utils/decomposedfs/testhelpers"
	"github.com/opencloud-eu/reva/v2/pkg/storagespace"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
)

var _ = Describe("Spaces", func() {

	Describe("Create Space", func() {
		var (
			env *helpers.TestEnv
		)
		BeforeEach(func() {
			var err error
			env, err = helpers.NewTestEnv(nil)
			Expect(err).ToNot(HaveOccurred())
			env.PermissionsClient.On("CheckPermission", mock.Anything, mock.Anything, mock.Anything).Return(
				func(ctx context.Context, in *cs3permissions.CheckPermissionRequest, opts ...grpc.CallOption) *cs3permissions.CheckPermissionResponse {
					if in.Permission == "Drives.DeletePersonal" && ctxpkg.ContextMustGetUser(ctx).Id.GetOpaqueId() == env.DeleteHomeSpacesUser.Id.OpaqueId {
						return &cs3permissions.CheckPermissionResponse{Status: &rpcv1beta1.Status{Code: rpcv1beta1.Code_CODE_OK}}
					}
					if in.Permission == "Drives.DeleteProject" && ctxpkg.ContextMustGetUser(ctx).Id.GetOpaqueId() == env.DeleteAllSpacesUser.Id.OpaqueId {
						return &cs3permissions.CheckPermissionResponse{Status: &rpcv1beta1.Status{Code: rpcv1beta1.Code_CODE_OK}}
					}
					if (in.Permission == "Drives.Create" || in.Permission == "Drives.List") && ctxpkg.ContextMustGetUser(ctx).Id.GetOpaqueId() == helpers.OwnerID {
						return &cs3permissions.CheckPermissionResponse{Status: &rpcv1beta1.Status{Code: rpcv1beta1.Code_CODE_OK}}
					}
					// any other user
					return &cs3permissions.CheckPermissionResponse{Status: &rpcv1beta1.Status{Code: rpcv1beta1.Code_CODE_PERMISSION_DENIED}}
				},
				nil)
			env.Permissions.On("AssemblePermissions", mock.Anything, mock.Anything, mock.Anything).Return(func(ctx context.Context, n *node.Node) *provider.ResourcePermissions {
				if ctxpkg.ContextMustGetUser(ctx).Id.GetOpaqueId() == "25b69780-5f39-43be-a7ac-a9b9e9fe4230" {
					return node.OwnerPermissions() // id of owner/admin
				}
				return node.NoPermissions()
			}, nil)
		})

		AfterEach(func() {
			if env != nil {
				env.Cleanup()
			}
		})

		Context("during login", func() {
			It("space is created", func() {
				resp, err := env.Fs.ListStorageSpaces(env.Ctx, nil, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(resp)).To(Equal(1))
				Expect(string(resp[0].Opaque.GetMap()["spaceAlias"].Value)).To(Equal("personal/username"))
				Expect(resp[0].Name).To(Equal("username"))
				Expect(resp[0].SpaceType).To(Equal("personal"))
			})
		})
		Context("when creating a space", func() {
			It("project space is created", func() {
				env.Owner = nil
				resp, err := env.Fs.CreateStorageSpace(env.Ctx, &provider.CreateStorageSpaceRequest{Name: "Mission to Mars", Type: "project"})
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.Status.Code).To(Equal(rpcv1beta1.Code_CODE_OK))
				Expect(resp.StorageSpace).ToNot(Equal(nil))
				Expect(string(resp.StorageSpace.Opaque.Map["spaceAlias"].Value)).To(Equal("project/mission-to-mars"))
				Expect(resp.StorageSpace.Name).To(Equal("Mission to Mars"))
				Expect(resp.StorageSpace.SpaceType).To(Equal("project"))
			})
		})

		Context("needs to check node permissions", func() {
			It("returns false on requesting for other user with canlistallspaces und no unrestricted privilege", func() {
				resp := env.Fs.MustCheckNodePermissions(env.Ctx, false)
				Expect(resp).To(Equal(true))
			})
			It("returns true on requesting unrestricted as non-admin", func() {
				ctx := ctxpkg.ContextSetUser(context.Background(), env.Users[0])
				resp := env.Fs.MustCheckNodePermissions(ctx, true)
				Expect(resp).To(Equal(true))
			})
			It("returns true on requesting for own spaces", func() {
				ctx := ctxpkg.ContextSetUser(context.Background(), env.Users[0])
				resp := env.Fs.MustCheckNodePermissions(ctx, false)
				Expect(resp).To(Equal(true))
			})
			It("returns false on unrestricted", func() {
				resp := env.Fs.MustCheckNodePermissions(env.Ctx, true)
				Expect(resp).To(Equal(false))
			})
		})

		Context("can delete homespace", func() {
			It("fails on trying to delete a homespace as non-admin", func() {
				ctx := ctxpkg.ContextSetUser(context.Background(), env.Users[1])
				resp, err := env.Fs.ListStorageSpaces(env.Ctx, nil, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(resp)).To(Equal(1))
				Expect(string(resp[0].Opaque.GetMap()["spaceAlias"].Value)).To(Equal("personal/username"))
				Expect(resp[0].Name).To(Equal("username"))
				Expect(resp[0].SpaceType).To(Equal("personal"))
				err = env.Fs.DeleteStorageSpace(ctx, &provider.DeleteStorageSpaceRequest{
					Id: resp[0].GetId(),
				})
				Expect(err).To(HaveOccurred())
			})
			It("succeeds on trying to delete homespace as user with 'delete-all-home-spaces' permission", func() {
				ctx := ctxpkg.ContextSetUser(context.Background(), env.DeleteHomeSpacesUser)
				resp, err := env.Fs.ListStorageSpaces(env.Ctx, nil, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(resp)).To(Equal(1))
				Expect(string(resp[0].Opaque.GetMap()["spaceAlias"].Value)).To(Equal("personal/username"))
				Expect(resp[0].Name).To(Equal("username"))
				Expect(resp[0].SpaceType).To(Equal("personal"))
				err = env.Fs.DeleteStorageSpace(ctx, &provider.DeleteStorageSpaceRequest{
					Id: resp[0].GetId(),
				})
				Expect(err).To(Not(HaveOccurred()))
			})
			It("fails on trying to delete homespace as user with 'delete-all-spaces' permission", func() {
				ctx := ctxpkg.ContextSetUser(context.Background(), env.DeleteAllSpacesUser)
				resp, err := env.Fs.ListStorageSpaces(env.Ctx, nil, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(resp)).To(Equal(1))
				Expect(string(resp[0].Opaque.GetMap()["spaceAlias"].Value)).To(Equal("personal/username"))
				Expect(resp[0].Name).To(Equal("username"))
				Expect(resp[0].SpaceType).To(Equal("personal"))
				err = env.Fs.DeleteStorageSpace(ctx, &provider.DeleteStorageSpaceRequest{
					Id: resp[0].GetId(),
				})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("can delete (purge) project spaces", func() {
			var delReq *provider.DeleteStorageSpaceRequest
			BeforeEach(func() {
				resp, err := env.Fs.CreateStorageSpace(env.Ctx, &provider.CreateStorageSpaceRequest{Name: "Mission to Venus", Type: "project"})
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.Status.Code).To(Equal(rpcv1beta1.Code_CODE_OK))
				Expect(resp.StorageSpace).ToNot(Equal(nil))
				spaceID := resp.StorageSpace.GetId()
				err = env.Fs.DeleteStorageSpace(env.Ctx, &provider.DeleteStorageSpaceRequest{
					Id: spaceID,
				})
				Expect(err).To(Not(HaveOccurred()))
				delReq = &provider.DeleteStorageSpaceRequest{
					Opaque: &typesv1beta1.Opaque{
						Map: map[string]*typesv1beta1.OpaqueEntry{
							"purge": {
								Decoder: "plain",
								Value:   []byte("true"),
							},
						},
					},
					Id: spaceID,
				}
			})
			It("fails as a unprivileged user", func() {
				ctx := ctxpkg.ContextSetUser(context.Background(), env.Users[1])
				err := env.Fs.DeleteStorageSpace(ctx, delReq)
				Expect(err).To(HaveOccurred())
			})
			It("fails as a user with 'delete-all-home-spaces privilege", func() {
				ctx := ctxpkg.ContextSetUser(context.Background(), env.DeleteHomeSpacesUser)
				err := env.Fs.DeleteStorageSpace(ctx, delReq)
				Expect(err).To(HaveOccurred())
			})
			It("succeeds as a user with 'delete-all-spaces privilege", func() {
				ctx := ctxpkg.ContextSetUser(context.Background(), env.DeleteAllSpacesUser)
				err := env.Fs.DeleteStorageSpace(ctx, delReq)
				Expect(err).To(Not(HaveOccurred()))
			})
			It("succeeds as the space owner", func() {
				err := env.Fs.DeleteStorageSpace(env.Ctx, delReq)
				Expect(err).To(Not(HaveOccurred()))
			})
		})
	})

	Describe("List spaces with personal space permissions", func() {
		var (
			env          *helpers.TestEnv
			spaceadmin   *userv1beta1.User
			admin        *userv1beta1.User
			projectSpace *provider.StorageSpace
		)

		BeforeEach(func() {
			var err error
			env, err = helpers.NewTestEnv(nil)
			Expect(err).ToNot(HaveOccurred())

			spaceadmin = env.Users[0]
			admin = env.Users[1]

			// space permissions granted to the users by their role
			grantedPermissions := func(user *userv1beta1.User) []string {
				switch user.GetId().GetOpaqueId() {
				case spaceadmin.GetId().GetOpaqueId():
					return []string{"Drives.Create", "Drives.List", "Drives.ReadWrite"}
				case admin.GetId().GetOpaqueId():
					return []string{"Drives.Create", "Drives.List", "Drives.ListPersonal", "Drives.ReadWrite", "Drives.ReadWritePersonal"}
				case env.Owner.GetId().GetOpaqueId():
					return []string{"Drives.Create"} // setup actor
				case env.DeleteAllSpacesUser.GetId().GetOpaqueId():
					return []string{"Drives.ListPersonal"} // custom role without Drives.List
				}
				return []string{}
			}
			env.PermissionsClient.On("CheckPermission", mock.Anything, mock.Anything, mock.Anything).Return(
				func(ctx context.Context, in *cs3permissions.CheckPermissionRequest, opts ...grpc.CallOption) *cs3permissions.CheckPermissionResponse {
					for _, permission := range grantedPermissions(ctxpkg.ContextMustGetUser(ctx)) {
						if permission == in.Permission {
							return &cs3permissions.CheckPermissionResponse{Status: &rpcv1beta1.Status{Code: rpcv1beta1.Code_CODE_OK}}
						}
					}
					return &cs3permissions.CheckPermissionResponse{Status: &rpcv1beta1.Status{Code: rpcv1beta1.Code_CODE_PERMISSION_DENIED}}
				},
				nil)

			env.Permissions.On("AssemblePermissions", mock.Anything, mock.Anything, mock.Anything).Unset()
			env.Permissions.On("AssemblePermissions", mock.Anything, mock.Anything, mock.Anything).Return(
				func(ctx context.Context, n *node.Node) *provider.ResourcePermissions {
					u := ctxpkg.ContextMustGetUser(ctx)
					switch u.GetId().GetOpaqueId() {
					case env.Owner.GetId().GetOpaqueId():
						return node.OwnerPermissions() // setup actor
					case spaceadmin.GetId().GetOpaqueId(), admin.GetId().GetOpaqueId():
						if n.SpaceID == u.GetId().GetOpaqueId() {
							return node.OwnerPermissions() // own personal space
						}
					}
					return node.NoPermissions()
				}, nil)

			createSpace := func(req *provider.CreateStorageSpaceRequest) *provider.StorageSpace {
				resp, err := env.Fs.CreateStorageSpace(env.Ctx, req)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.GetStatus().GetCode()).To(Equal(rpcv1beta1.Code_CODE_OK))
				return resp.GetStorageSpace()
			}
			// personal spaces of the other users, in addition to the owner's own one
			createSpace(&provider.CreateStorageSpaceRequest{Owner: spaceadmin, Name: "spaceadmin", Type: "personal"})
			createSpace(&provider.CreateStorageSpaceRequest{Owner: admin, Name: "admin", Type: "personal"})
			projectSpace = createSpace(&provider.CreateStorageSpaceRequest{Name: "Project A", Type: "project"})
			createSpace(&provider.CreateStorageSpaceRequest{Name: "Project B", Type: "project"})
		})

		AfterEach(func() {
			if env != nil {
				env.Cleanup()
			}
		})

		spaceNames := func(spaces []*provider.StorageSpace) []string {
			names := make([]string, 0, len(spaces))
			for _, space := range spaces {
				names = append(names, space.GetName())
			}
			return names
		}

		It("lists all project spaces but only the own personal space without Drives.ListPersonal", func() {
			spaces, err := env.Fs.ListStorageSpaces(ctxpkg.ContextSetUser(context.Background(), spaceadmin), nil, true)
			Expect(err).ToNot(HaveOccurred())
			Expect(spaceNames(spaces)).To(ConsistOf("spaceadmin", "Project A", "Project B"))
		})

		It("lists all spaces with Drives.ListPersonal", func() {
			spaces, err := env.Fs.ListStorageSpaces(ctxpkg.ContextSetUser(context.Background(), admin), nil, true)
			Expect(err).ToNot(HaveOccurred())
			Expect(spaceNames(spaces)).To(ConsistOf("username", "spaceadmin", "admin", "Project A", "Project B"))
		})

		It("only lists the own personal space when filtering by the personal space type", func() {
			spaces, err := env.Fs.ListStorageSpaces(ctxpkg.ContextSetUser(context.Background(), spaceadmin), []*provider.ListStorageSpacesRequest_Filter{
				{Type: provider.ListStorageSpacesRequest_Filter_TYPE_SPACE_TYPE, Term: &provider.ListStorageSpacesRequest_Filter_SpaceType{SpaceType: "personal"}},
			}, true)
			Expect(err).ToNot(HaveOccurred())
			Expect(spaceNames(spaces)).To(ConsistOf("spaceadmin"))
			Expect(spaces[0].GetSpaceType()).To(Equal("personal"))
		})

		It("only lists the own spaces when listing restricted", func() {
			spaces, err := env.Fs.ListStorageSpaces(ctxpkg.ContextSetUser(context.Background(), spaceadmin), nil, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(spaceNames(spaces)).To(ConsistOf("spaceadmin"))
		})

		It("only lists the personal spaces of another user with Drives.ListPersonal", func() {
			// the requested user's groups would normally be resolved via the gateway
			Expect(env.Fs.UserCache.Set(admin.GetId().GetOpaqueId(), admin)).To(Succeed())

			spaces, err := env.Fs.ListStorageSpaces(
				ctxpkg.ContextSetUser(context.Background(), env.DeleteAllSpacesUser),
				[]*provider.ListStorageSpacesRequest_Filter{
					{Type: provider.ListStorageSpacesRequest_Filter_TYPE_USER, Term: &provider.ListStorageSpacesRequest_Filter_User{User: admin.GetId()}},
				},
				true,
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(spaceNames(spaces)).To(ConsistOf("admin"))
		})

		It("space admin can read the quota of a project space", func() {
			_, _, _, err := env.Fs.GetQuota(ctxpkg.ContextSetUser(context.Background(), spaceadmin), &provider.Reference{ResourceId: projectSpace.GetRoot()})
			Expect(err).ToNot(HaveOccurred())
		})

		It("space admin cannot read the quota of another user's personal space", func() {
			_, _, _, err := env.Fs.GetQuota(ctxpkg.ContextSetUser(context.Background(), spaceadmin), &provider.Reference{ResourceId: env.SpaceRootRes})
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(errtypes.NotFound("")))
		})

		It("admin can read the quota of another user's personal space", func() {
			_, _, _, err := env.Fs.GetQuota(ctxpkg.ContextSetUser(context.Background(), admin), &provider.Reference{ResourceId: env.SpaceRootRes})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Create Space with custom alias template", func() {
		Describe("Create Spaces with custom alias template", func() {
			var (
				env *helpers.TestEnv
			)

			BeforeEach(func() {
				var err error
				env, err = helpers.NewTestEnv(map[string]interface{}{
					"personalspacealias_template": "{{.SpaceType}}/{{.Email.Local}}@{{.Email.Domain}}",
					"generalspacealias_template":  "{{.SpaceType}}:{{.SpaceName | replace \" \" \"-\" | upper}}",
				})
				Expect(err).ToNot(HaveOccurred())
				env.PermissionsClient.On("CheckPermission", mock.Anything, mock.Anything, mock.Anything).Return(&cs3permissions.CheckPermissionResponse{Status: &rpcv1beta1.Status{Code: rpcv1beta1.Code_CODE_OK}}, nil)
				env.Permissions.On("AssemblePermissions", mock.Anything, mock.Anything, mock.Anything).Return(&provider.ResourcePermissions{
					Stat:     true,
					AddGrant: true,
					GetQuota: true,
				}, nil)
			})

			AfterEach(func() {
				if env != nil {
					env.Cleanup()
				}
			})
			Context("during login", func() {
				It("personal space is created with custom alias", func() {
					resp, err := env.Fs.ListStorageSpaces(env.Ctx, nil, false)
					Expect(err).ToNot(HaveOccurred())
					Expect(len(resp)).To(Equal(1))
					Expect(string(resp[0].Opaque.GetMap()["spaceAlias"].Value)).To(Equal("personal/username@_unknown"))
				})
			})
			Context("creating a space", func() {
				It("project space is created with custom alias", func() {
					env.Owner = nil
					resp, err := env.Fs.CreateStorageSpace(env.Ctx, &provider.CreateStorageSpaceRequest{Name: "Mission to Venus", Type: "project"})
					Expect(err).ToNot(HaveOccurred())
					Expect(resp.Status.Code).To(Equal(rpcv1beta1.Code_CODE_OK))
					Expect(resp.StorageSpace).ToNot(Equal(nil))
					Expect(string(resp.StorageSpace.Opaque.Map["spaceAlias"].Value)).To(Equal("project:MISSION-TO-VENUS"))

				})
			})
		})
	})

	Describe("Update Space", func() {
		var (
			env     *helpers.TestEnv
			spaceid *provider.StorageSpaceId

			spaceadmin    *userv1beta1.User
			personalSpace *provider.StorageSpaceId

			manager  = &userv1beta1.User{Id: &userv1beta1.UserId{OpaqueId: "manager"}}
			editor   = &userv1beta1.User{Id: &userv1beta1.UserId{OpaqueId: "editor"}}
			viewer   = &userv1beta1.User{Id: &userv1beta1.UserId{OpaqueId: "viewer"}}
			nomember = &userv1beta1.User{Id: &userv1beta1.UserId{OpaqueId: "nomember"}}
			admin    = &userv1beta1.User{Id: &userv1beta1.UserId{OpaqueId: "admin"}}
		)
		BeforeEach(func() {
			var err error
			env, err = helpers.NewTestEnv(nil)
			Expect(err).ToNot(HaveOccurred())

			spaceadmin = env.Users[0]
			personalSpace = &provider.StorageSpaceId{OpaqueId: storagespace.FormatResourceID(env.SpaceRootRes)}

			// space permissions
			env.PermissionsClient.On("CheckPermission", mock.Anything, mock.Anything, mock.Anything).Return(
				func(ctx context.Context, in *cs3permissions.CheckPermissionRequest, opts ...grpc.CallOption) *cs3permissions.CheckPermissionResponse {
					switch ctxpkg.ContextMustGetUser(ctx).GetId().GetOpaqueId() {
					case manager.GetId().GetOpaqueId():
						switch in.Permission {
						case "Drives.Create":
							return &cs3permissions.CheckPermissionResponse{Status: &rpcv1beta1.Status{Code: rpcv1beta1.Code_CODE_OK}}
						default:
							return &cs3permissions.CheckPermissionResponse{Status: &rpcv1beta1.Status{Code: rpcv1beta1.Code_CODE_PERMISSION_DENIED}}
						}
					case spaceadmin.GetId().GetOpaqueId():
						switch in.Permission {
						case "Drives.ReadWrite":
							return &cs3permissions.CheckPermissionResponse{Status: &rpcv1beta1.Status{Code: rpcv1beta1.Code_CODE_OK}}
						default:
							return &cs3permissions.CheckPermissionResponse{Status: &rpcv1beta1.Status{Code: rpcv1beta1.Code_CODE_PERMISSION_DENIED}}
						}
					case admin.GetId().GetOpaqueId():
						return &cs3permissions.CheckPermissionResponse{Status: &rpcv1beta1.Status{Code: rpcv1beta1.Code_CODE_OK}}
					default:
						return &cs3permissions.CheckPermissionResponse{Status: &rpcv1beta1.Status{Code: rpcv1beta1.Code_CODE_PERMISSION_DENIED}}
					}
				}, nil)

			env.Permissions.On("AssemblePermissions", mock.Anything, mock.Anything, mock.Anything).Unset()
			env.Permissions.On("AssemblePermissions", mock.Anything, mock.Anything, mock.Anything).Return(
				func(ctx context.Context, n *node.Node) *provider.ResourcePermissions {
					switch ctxpkg.ContextMustGetUser(ctx).GetId().GetOpaqueId() {
					case manager.GetId().GetOpaqueId():
						return node.OwnerPermissions() // id of owner/admin
					case env.Owner.GetId().GetOpaqueId():
						return node.OwnerPermissions() // owner of the personal space
					case editor.GetId().GetOpaqueId():
						return &provider.ResourcePermissions{InitiateFileUpload: true} // mock editor
					case viewer.GetId().GetOpaqueId():
						return &provider.ResourcePermissions{Stat: true} // mock viewer
					default:
						return node.NoPermissions()
					}
				}, nil)

			resp, err := env.Fs.CreateStorageSpace(ctxpkg.ContextSetUser(context.Background(), manager), &provider.CreateStorageSpaceRequest{Name: "Mission to Venus", Type: "project"})
			Expect(err).ToNot(HaveOccurred())

			spaceid = resp.GetStorageSpace().GetId()
		})

		AfterEach(func() {
			if env != nil {
				env.Cleanup()
			}
		})

		DescribeTable("update project spaces",
			func(details func() (*userv1beta1.User, *provider.UpdateStorageSpaceRequest), expectedStatusCode rpcv1beta1.Code) {
				u, req := details()
				r, err := env.Fs.UpdateStorageSpace(ctxpkg.ContextSetUser(context.Background(), u), req)
				Expect(err).ToNot(HaveOccurred())
				Expect(r.Status.Code).To(Equal(expectedStatusCode))
			},

			Entry("Manager can change everything but quota",
				func() (*userv1beta1.User, *provider.UpdateStorageSpaceRequest) {
					return manager, &provider.UpdateStorageSpaceRequest{
						StorageSpace: &provider.StorageSpace{
							Name: "new space name",
							Id:   spaceid,
							Opaque: &typesv1beta1.Opaque{
								Map: map[string]*typesv1beta1.OpaqueEntry{
									"description": {
										Decoder: "plain",
										Value:   []byte("new description"),
									},
									"spacealias": {
										Decoder: "plain",
										Value:   []byte("new alias"),
									},
									"image": {
										Decoder: "plain",
										Value:   []byte("a$b!c"),
									},
									"readme": {
										Decoder: "plain",
										Value:   []byte("f$g!h"),
									},
								},
							},
						},
					}
				},
				rpcv1beta1.Code_CODE_OK,
			),
			Entry("Manager cannot change quota, even with large request",
				func() (*userv1beta1.User, *provider.UpdateStorageSpaceRequest) {
					return manager, &provider.UpdateStorageSpaceRequest{
						StorageSpace: &provider.StorageSpace{
							Name:  "new space name",
							Id:    spaceid,
							Quota: &provider.Quota{QuotaMaxBytes: uint64(1000)},
							Opaque: &typesv1beta1.Opaque{
								Map: map[string]*typesv1beta1.OpaqueEntry{
									"description": {
										Decoder: "plain",
										Value:   []byte("new description"),
									},
									"spacealias": {
										Decoder: "plain",
										Value:   []byte("new alias"),
									},
									"image": {
										Decoder: "plain",
										Value:   []byte("a$b!c"),
									},
									"readme": {
										Decoder: "plain",
										Value:   []byte("f$g!h"),
									},
								},
							},
						},
					}
				},
				rpcv1beta1.Code_CODE_PERMISSION_DENIED,
			),
			Entry("Editor cannot change quota",
				func() (*userv1beta1.User, *provider.UpdateStorageSpaceRequest) {
					return editor, &provider.UpdateStorageSpaceRequest{
						StorageSpace: &provider.StorageSpace{
							Id:    spaceid,
							Quota: &provider.Quota{QuotaMaxBytes: uint64(1000)},
						},
					}
				},
				rpcv1beta1.Code_CODE_PERMISSION_DENIED,
			),
			Entry("Editor cannot change name",
				func() (*userv1beta1.User, *provider.UpdateStorageSpaceRequest) {
					return editor, &provider.UpdateStorageSpaceRequest{
						StorageSpace: &provider.StorageSpace{
							Name: "new spacename",
							Id:   spaceid,
						},
					}
				},
				rpcv1beta1.Code_CODE_PERMISSION_DENIED,
			),
			Entry("Admin can change quota",
				func() (*userv1beta1.User, *provider.UpdateStorageSpaceRequest) {
					return admin, &provider.UpdateStorageSpaceRequest{
						StorageSpace: &provider.StorageSpace{
							Id:    spaceid,
							Quota: &provider.Quota{QuotaMaxBytes: uint64(1000)},
						},
					}
				},
				rpcv1beta1.Code_CODE_OK,
			),
			Entry("Admin can change name and description",
				func() (*userv1beta1.User, *provider.UpdateStorageSpaceRequest) {
					return admin, &provider.UpdateStorageSpaceRequest{
						StorageSpace: &provider.StorageSpace{
							Name: "new spacename",
							Id:   spaceid,
							Opaque: &typesv1beta1.Opaque{
								Map: map[string]*typesv1beta1.OpaqueEntry{
									"description": {
										Decoder: "plain",
										Value:   []byte("new description"),
									},
								},
							},
						},
					}
				},
				rpcv1beta1.Code_CODE_OK,
			),
			Entry("Viewer gets OK when he changes nothing",
				func() (*userv1beta1.User, *provider.UpdateStorageSpaceRequest) {
					return viewer, &provider.UpdateStorageSpaceRequest{
						StorageSpace: &provider.StorageSpace{
							Id: spaceid,
						},
					}
				},
				rpcv1beta1.Code_CODE_OK,
			),
			Entry("NoMember will not find the space",
				func() (*userv1beta1.User, *provider.UpdateStorageSpaceRequest) {
					return nomember, &provider.UpdateStorageSpaceRequest{
						StorageSpace: &provider.StorageSpace{
							Id: spaceid,
						},
					}
				},
				rpcv1beta1.Code_CODE_NOT_FOUND,
			),
		)

		DescribeTable("update personal spaces",
			func(details func() (*userv1beta1.User, *provider.UpdateStorageSpaceRequest), expectedStatusCode rpcv1beta1.Code) {
				u, req := details()
				r, err := env.Fs.UpdateStorageSpace(ctxpkg.ContextSetUser(context.Background(), u), req)
				Expect(err).ToNot(HaveOccurred())
				Expect(r.Status.Code).To(Equal(expectedStatusCode))
			},

			Entry("SpaceAdmin cannot change the name of a personal space",
				func() (*userv1beta1.User, *provider.UpdateStorageSpaceRequest) {
					return spaceadmin, &provider.UpdateStorageSpaceRequest{
						StorageSpace: &provider.StorageSpace{
							Name: "new personal space name",
							Id:   personalSpace,
						},
					}
				},
				rpcv1beta1.Code_CODE_PERMISSION_DENIED,
			),
			Entry("SpaceAdmin cannot change the description of a personal space",
				func() (*userv1beta1.User, *provider.UpdateStorageSpaceRequest) {
					return spaceadmin, &provider.UpdateStorageSpaceRequest{
						StorageSpace: &provider.StorageSpace{
							Id: personalSpace,
							Opaque: &typesv1beta1.Opaque{
								Map: map[string]*typesv1beta1.OpaqueEntry{
									"description": {
										Decoder: "plain",
										Value:   []byte("new description"),
									},
								},
							},
						},
					}
				},
				rpcv1beta1.Code_CODE_PERMISSION_DENIED,
			),
			Entry("Admin can change the name of a personal space",
				func() (*userv1beta1.User, *provider.UpdateStorageSpaceRequest) {
					return admin, &provider.UpdateStorageSpaceRequest{
						StorageSpace: &provider.StorageSpace{
							Name: "new personal space name",
							Id:   personalSpace,
						},
					}
				},
				rpcv1beta1.Code_CODE_OK,
			),
			Entry("Owner can change the name of his personal space",
				func() (*userv1beta1.User, *provider.UpdateStorageSpaceRequest) {
					return env.Owner, &provider.UpdateStorageSpaceRequest{
						StorageSpace: &provider.StorageSpace{
							Name: "new personal space name",
							Id:   personalSpace,
						},
					}
				},
				rpcv1beta1.Code_CODE_OK,
			),
			Entry("SpaceAdmin can change the name of a project space",
				func() (*userv1beta1.User, *provider.UpdateStorageSpaceRequest) {
					return spaceadmin, &provider.UpdateStorageSpaceRequest{
						StorageSpace: &provider.StorageSpace{
							Name: "new space name",
							Id:   spaceid,
						},
					}
				},
				rpcv1beta1.Code_CODE_OK,
			),
		)
	})
})
