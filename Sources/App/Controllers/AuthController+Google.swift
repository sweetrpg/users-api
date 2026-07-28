//
// AuthController+Google.swift
// Copyright (c) 2021 Paul Schifferer.
//

// import ImperialGoogle
// import Vapor

// extension AuthController {
//     static let googleAuthTypeId = "google"

//     func addGoogle(routes : RoutesBuilder) throws {
//         guard let googleCallbackURL = Environment.get("GOOGLE_CALLBACK_URL") else {
//             fatalError("Google callback URL not set")
//         }

//         try routes.oAuth(from: Google.self,
//                 authenticate: "auth/login/google",
//                 callback: googleCallbackURL,
//                 scope: [ "profile", "email" ],
//                 completion: self.processGoogleLogin)
//         routes.get(Self.endpointPath, "login", "ios", use: self.iosGoogleLogin)
//     }

//     func processGoogleLogin(request : Request, token _ : String) throws -> EventLoopFuture<ResponseEncodable> {
//         try Google.getUser(on: request)
//                   .flatMap { userInfo in
//                       User.query(on: request.db)
//                           .filter(\.$username, .equal, userInfo.email)
//                           .first()
//                           .flatMap { foundUser in
//                               guard let existingUser = foundUser else {
//                                   let user = User(name: userInfo.name,
//                                           username: userInfo.email,
// //                                          password: "",
//                                           thirdPartyAuth: Self.googleAuthTypeId,
//                                           thirdPartyAuthId: userInfo.email,
//                                           email: userInfo.email)
//                                   return user.save(on: request.db)
//                                              .flatMap {
//                                                  request.session.authenticate(user)
//                                                  return generateRedirect(on: request, for: user)
//                                              }
//                               }

//                               request.session.authenticate(existingUser)
//                               return generateRedirect(on: request, for: existingUser)
//                           }
//                   }
//     }

//     func iosGoogleLogin(_ req : Request) -> Response {
//         req.session.data[Constants.oauthLoginDataKey] = Constants.iosLoginType
//         return req.redirect(to: "/auth/login/google")
//     }
// }

// // MARK: - GoogleUserInfo

// struct GoogleUserInfo : Content {
//     let email : String
//     let name : String
// }

// extension Google {
//     static func getUser(on request : Request) throws -> EventLoopFuture<GoogleUserInfo> {
//         var headers = HTTPHeaders()
//         headers.bearerAuthorization = try BearerAuthorization(token: request.accessToken())

//         let googleAPIURL : URI = "https://www.googleapis.com/oauth2/v1/userinfo?alt=json"
//         return request
//                 .client
//                 .get(googleAPIURL, headers: headers)
//                 .flatMapThrowing { response in
//                     guard response.status == .ok else {
//                         if response.status == .unauthorized {
//                             throw Abort.redirect(to: "/auth/login/google")
//                         }
//                         else {
//                             throw Abort(.internalServerError)
//                         }
//                     }

//                     return try response.content
//                             .decode(GoogleUserInfo.self)
//                 }
//     }
// }
