//
// UsersController.swift
// Copyright (c) 2021 Paul Schifferer.
//

import Fluent
import UserModel
// import JWT
import Vapor

struct ProfilesController: RouteCollection {
  static let endpointPath: PathComponent = "profiles"

  func boot(routes: RoutesBuilder) throws {
    let group = routes.grouped(Constants.apiPath, Self.endpointPath)

    // group.get(use: self.getAllHandler)
    // group.get(":userId", use: self.getHandler)
    //        group.put(":userId", use: updateHandler)
    //        group.delete(":userId", use: deleteHandler)
    //        group.get(":userId", "acronyms", use: self.getAcronymsHandler)

    //        let basicAuthMiddleware = User.authenticator()
    //        let basicProtected = group.grouped(basicAuthMiddleware)
    //        basicProtected.post("login", use: self.loginHandler)

    // let tokenAuthMiddleware = Token.authenticator()
    // let guardAuthMiddleware = User.guardMiddleware()
    // let tokenProtected = group.grouped(tokenAuthMiddleware, guardAuthMiddleware)
    // tokenProtected.post(use: self.createHandler)
  }

  //     func createHandler(_ req : Request) throws -> EventLoopFuture<User.Public> {
  //         let user = try req.content.decode(User.self)
  // //        user.password = try Bcrypt.hash(user.password)
  //         return user.save(on: req.db)
  //                    .map {
  //                        user.convertToPublic()
  //                    }
  //     }

  //     func getAllHandler(_ req : Request) -> EventLoopFuture<[User.Public]> {
  //         User.query(on: req.db)
  //             .sort(\.$username)
  //             .all()
  //             .convertToPublic()
  //     }

  //     func getHandler(_ req : Request) -> EventLoopFuture<User.Public> {
  //         User.find(
  //                     req.parameters.get("userId"),
  //                     on: req.db
  //             )
  //             .unwrap(or: Abort(.notFound))
  //             .convertToPublic()
  //     }

  //     //    func updateHandler(_ req : Request) -> EventLoopFuture<User> {
  //     //        <#code#>
  //     //    }

  // //    func getAcronymsHandler(_ req : Request) -> EventLoopFuture<[Acronym]> {
  // //        User.find(
  // //                    req.parameters.get("userId"),
  // //                    on: req.db
  // //            )
  // //            .unwrap(or: Abort(.notFound))
  // //            .flatMap { user in
  // //                user.$acronyms.get(on: req.db)
  // //            }
  // //    }

  //     func loginHandler(_ req : Request) throws -> EventLoopFuture<Token> {
  //         let user = try req.auth.require(User.self)
  //         let token = try Token.generate(for: user)
  //         return token.save(on: req.db)
  //                     .map {
  //                         token
  //                     }
  //     }
}
