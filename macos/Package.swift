// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "Hotseat",
    platforms: [.macOS(.v14)],
    targets: [
        .target(name: "HotseatKit"),
        .executableTarget(name: "Hotseat", dependencies: ["HotseatKit"]),
        .testTarget(name: "HotseatKitTests", dependencies: ["HotseatKit"]),
    ]
)
