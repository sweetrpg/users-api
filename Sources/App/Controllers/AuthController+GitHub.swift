//
// AuthController+GitHub.swift
// Copyright (c) 2021 Paul Schifferer.
//

// import ImperialGitHub
// import Vapor

// extension AuthController {
//     static let githubAuthTypeId = "github"

//     func addGitHub(routes : RoutesBuilder) throws {
//         guard let githubCallbackURL = Environment.get("GITHUB_CALLBACK_URL") else {
//             fatalError("GitHub callback URL not set")
//         }

//         try routes.oAuth(from: GitHub.self,
//                 authenticate: "auth/login/github",
//                 callback: githubCallbackURL,
//                 scope: [ "user:email" ],
//                 completion: self.processGitHubLogin)
// //        routes.get(Self.endpointPath, "login", "ios", use: iosGoogleLogin)
//     }

//     func processGitHubLogin(request : Request, token _ : String) throws -> EventLoopFuture<ResponseEncodable> {
//         try GitHub.getUser(on: request)
//                   .and(GitHub.getEmails(on: request))
//                   .flatMap { userInfo, emailInfo in
//                       User.query(on: request.db)
//                           .filter(\.$username, .equal, userInfo.login)
//                           .first()
//                           .flatMap { foundUser in
//                               guard let existingUser = foundUser else {
//                                   let user = User(name: userInfo.name,
//                                           username: userInfo.login,
// //                                          password: "",
//                                           thirdPartyAuth: Self.githubAuthTypeId,
//                                           thirdPartyAuthId: userInfo.login,
//                                           email: emailInfo[0].email)
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

// //    func iosGoogleLogin(_ req : Request) -> Response {
// //        req.session.data[Constants.oauthLoginDataKey] = Constants.iosLoginType
// //        return req.redirect(to: "/auth/login/google")
// //    }
// }

// // MARK: - GitHubUserInfo

// struct GitHubUserInfo : Content {
//     let name : String
//     let login : String
// }

// struct GitHubEmailInfo : Content {
//     let email : String
// }

// extension GitHub {
//     static func getUser(on request : Request) throws -> EventLoopFuture<GitHubUserInfo> {
//         var headers = HTTPHeaders()
//         try headers.add(name: .authorization, value: "token \(try request.accessToken())")
//         headers.add(name: .userAgent, value: "vapor")

//         let githubUserAPIURL : URI = "https://api.github.com/user"
//         return request
//                 .client
//                 .get(githubUserAPIURL, headers: headers)
//                 .flatMapThrowing { response in
//                     guard response.status == .ok else {
//                         if response.status == .unauthorized {
//                             throw Abort.redirect(to: "/auth/login/github")
//                         }
//                         else {
//                             throw Abort(.internalServerError)
//                         }
//                     }

//                     return try response.content
//                             .decode(GitHubUserInfo.self)
//                 }
//     }

//     static func getEmails(on request : Request) throws -> EventLoopFuture<[GitHubEmailInfo]> {
//         var headers = HTTPHeaders()
//         try headers.add(name: .authorization, value: "token \(try request.accessToken())")
//         headers.add(name: .userAgent, value: "TILApp")

//         let githubUserAPIURL : URI = "https://api.github.com/user/emails"
//         return request.client
//                 .get(githubUserAPIURL, headers: headers)
//                 .flatMapThrowing { response in
//                     guard response.status == .ok else {
//                         if response.status == .unauthorized {
//                             throw Abort.redirect(to: "/auth/login/github")
//                         }
//                         else {
//                             throw Abort(.internalServerError)
//                         }
//                     }
//                     return try response.content
//                             .decode([ GitHubEmailInfo ].self)
//                 }
//     }
// }
