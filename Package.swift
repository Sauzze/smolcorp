// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "AgentDeck",
    platforms: [.macOS(.v13)],
    targets: [
        .executableTarget(name: "AgentDeck", path: "Sources/AgentDeck"),
        // The parts that read and write your files, tested on files of their own.
        .testTarget(name: "AgentDeckTests", dependencies: ["AgentDeck"], path: "Tests/AgentDeckTests"),
    ]
)
