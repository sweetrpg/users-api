import Vapor

// import SecurityModel

/// Creates new users and logs them in.
final class UsersController: RouteCollection {
  static let endpointPath: PathComponent = "users"

  func boot(routes: RoutesBuilder) throws {
    let group = routes.grouped(Constants.apiPath, Self.endpointPath)
  }

  // func loginHandler(_ req : Request) throws -> Future<Token> {

  // }
}
