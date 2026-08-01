//
// AuthController.swift
// Copyright (c) 2021 Paul Schifferer.
//

import Fluent
import UserModel
// import ImperialGoogle
import Vapor

struct AuthController: RouteCollection {
  static let endpointPath: PathComponent = "auth"

  func boot(routes: RoutesBuilder) throws {
    // try addGoogle(routes: routes)
    // try addGitHub(routes: routes)
    try addAuth0(routes: routes)
  }

  func generateRedirect(on req: Request, for user: User) -> EventLoopFuture<ResponseEncodable> {
    let redirectURL: EventLoopFuture<String>

    //        if req.session.data[Constants.oauthLoginDataKey] == Constants.iosLoginType {
    //            do {
    //                let token = try Token.generate(for: user)
    //                redirectURL = token.save(on: req.db)
    //                                   .map {
    //                                       "tilapp://auth?token=\(token.value)"
    //                                   }
    //            }
    //            catch {
    //                return req.eventLoop.future(error: error)
    //            }
    //        }
    //        else {
    redirectURL = req.eventLoop.future("/")
    //        }

    req.session.data[Constants.oauthLoginDataKey] = nil

    return redirectURL.map { url in
      req.redirect(to: url)
    }
  }
}
