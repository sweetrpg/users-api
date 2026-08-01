//
// AuthController+Auth0.swift
// Copyright (c) 2021 Paul Schifferer.
//

//import ImperialAuth0
import Vapor

extension AuthController {
  static let auth0AuthTypeId = "auth0"

  func addAuth0(routes: RoutesBuilder) throws {
    //        guard let auth0CallbackURL = Environment.get("AUTH0_CALLBACK_URL") else {
    //            fatalError("Auth0 callback URL not set")
    //        }
    //
    //        try routes.oAuth(from: Auth0.self,
    //                authenticate: "auth/login/auth0",
    //                callback: auth0CallbackURL,
    //                completion: self.processAuth0Login)
    ////        routes.get(Self.endpointPath, "login", "ios", use: iosGoogleLogin)
  }
  //
  //    func processAuth0Login(request : Request, token _ : String) throws -> EventLoopFuture<ResponseEncodable> {
  //        try Auth0.getUser(on: request)
  //                 .flatMap { userInfo in
  //                     User.query(on: request.db)
  //                         .filter(\.$username, .equal, userInfo.email)
  //                         .first()
  //                         .flatMap { foundUser in
  //                             guard let existingUser = foundUser else {
  //                                 let user = User(name: userInfo.name,
  //                                         username: userInfo.email,
  ////                                         password: "",
  //                                         thirdPartyAuth: Self.auth0AthTypeId,
  //                                         thirdPartyAuthId: userInfo.email,
  //                                         email: "TODO@TODO")
  //                                 return user.save(on: request.db)
  //                                            .flatMap {
  //                                                request.session.authenticate(user)
  //                                                return generateRedirect(on: request, for: user)
  //                                            }
  //                             }
  //
  //                             request.session.authenticate(existingUser)
  //                             return generateRedirect(on: request, for: existingUser)
  //                         }
  //                 }
  //    }
  //
  ////    func iosGoogleLogin(_ req : Request) -> Response {
  ////        req.session.data[Constants.oauthLoginDataKey] = Constants.iosLoginType
  ////        return req.redirect(to: "/auth/login/google")
  ////    }
}
//
//// MARK: - Auth0UserInfo
//
//struct Auth0UserInfo : Content {
//    let email : String
//    let name : String
//}
//
//extension Auth0 {
//    static func getUser(on request : Request) throws -> EventLoopFuture<Auth0UserInfo> {
//        var headers = HTTPHeaders()
//        headers.bearerAuthorization = try BearerAuthorization(token: request.accessToken())
//
//        let auth0APIURL : URI = "https://www.googleapis.com/oauth2/v1/userinfo?alt=json"
//        return request
//                .client
//                .get(auth0APIURL, headers: headers)
//                .flatMapThrowing { response in
//                    guard response.status == .ok else {
//                        if response.status == .unauthorized {
//                            throw Abort.redirect(to: "/auth/login/auth0")
//                        }
//                        else {
//                            throw Abort(.internalServerError)
//                        }
//                    }
//
//                    return try response.content
//                            .decode(Auth0UserInfo.self)
//                }
//    }
//}
