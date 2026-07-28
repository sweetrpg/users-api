//
// routes.swift
// Copyright (c) 2021 Paul Schifferer.
//

import Common
import Fluent
import Vapor

func routes(_ app: Application) throws {
  try app.register(collection: UsersController())
  try app.register(collection: AuthController())
  try app.register(collection: AuthzController())
}
