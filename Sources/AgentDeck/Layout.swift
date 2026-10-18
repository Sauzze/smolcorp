import CoreGraphics
import Foundation

/// How the phone's deck is laid out, as set up on the setup page: how many
/// session keys, whether the office shows in their place and how it's
/// furnished, whether the dial and stick show, and the rows of buttons under
/// them. It is checked before it's kept: the phone draws it, and its
/// buttons type into sessions. The Windows and Linux app's layout.go matches.
struct Layout: Codable, Equatable {
    var keys = KeyCount.auto
    /// The Agent Office in place of the keys: a desk for each, in the same order.
    var office = false
    var room = Room()
    var dial = true
    var stick = true
    var rows: [ButtonRow] = []

    static let maxRows = 4
    static let maxPerRow = 4

    /// Today's deck: a key per session, the dial and stick, then ✓ ✕ and
    /// the mic over esc, send and /clear.
    static let standard = Layout(rows: [
        ButtonRow(tall: true, buttons: ["accept", "reject", "mic"].map { Button(id: $0) }),
        ButtonRow(tall: false, buttons: ["esc", "send", "clear"].map { Button(id: $0) }),
    ])

    init(keys: KeyCount = .auto, office: Bool = false, room: Room = Room(), dial: Bool = true, stick: Bool = true, rows: [ButtonRow] = []) {
        self.keys = keys
        self.office = office
        self.room = room
        self.dial = dial
        self.stick = stick
        self.rows = rows
    }

    /// What's left out is the default: Auto keys, no office, a standard
    /// room, the dial and the stick showing, no buttons. A value of the wrong
    /// kind refuses the layout, except in the room, which only changes how
    /// the office looks.
    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        keys = try c.decodeIfPresent(KeyCount.self, forKey: .keys) ?? .auto
        office = try c.decodeIfPresent(Bool.self, forKey: .office) ?? false
        room = (try? c.decodeIfPresent(Room.self, forKey: .room)) ?? Room()
        dial = try c.decodeIfPresent(Bool.self, forKey: .dial) ?? true
        stick = try c.decodeIfPresent(Bool.self, forKey: .stick) ?? true
        rows = try c.decodeIfPresent([ButtonRow].self, forKey: .rows) ?? []
    }

    /// A layout sent by the setup page or found in a profile's file, checked.
    static func parse(_ data: Data) -> Layout? {
        (try? JSONDecoder().decode(Layout.self, from: data))?.checked
    }

    /// What the deck can draw and press: known buttons, each of the deck's
    /// own once, within the limits.
    var checked: Layout {
        var seen = Set<String>()
        let kept = rows.prefix(Layout.maxRows).compactMap { row -> ButtonRow? in
            var buttons: [Button] = []
            for case let button? in row.buttons.map(\.checked)
            where !seen.contains(button.id) && buttons.count < Layout.maxPerRow {
                seen.insert(button.id)
                buttons.append(button)
            }
            return buttons.isEmpty ? nil : ButtonRow(tall: row.tall, buttons: buttons)
        }
        return Layout(keys: keys, office: office, room: room, dial: dial, stick: stick, rows: kept)
    }

    /// The button made in setup with `id`, if the layout has it.
    func madeButton(_ id: String) -> Button? {
        rows.lazy.flatMap(\.buttons).first { $0.id == id && !Button.builtIn.contains($0.id) }
    }

    /// As JSON's own types, for a reply built with JSONSerialization.
    var jsonObject: Any {
        (try? JSONSerialization.jsonObject(with: JSONEncoder().encode(self))) ?? [:]
    }
}

/// How the office is furnished: what's talked into, what each of
/// the back wall's five places holds, the floor, the walls' paint, the
/// lounge's carpet and the piece in its corner, whether the sky keeps the
/// computer's time or it's always day, its pets, whether the windows follow
/// the seasons, how loud it is, the desks' finish, what everyone works at (a
/// desk, a tent, a throne…), the monitors and the chairs' colour, how many of
/// its own staff hang about, the style it's in (Yours, or one that furnishes
/// it for you), your team, the dress code, and where everything goes. office.js
/// draws it, with the same choices. Anything unknown, or of the wrong kind, is
/// as standard: it only changes how the room looks.
struct Room: Codable, Equatable {
    var mic = "mic"
    var wall = Room.standardWall
    var floor = "wood"
    var paint = "slate"
    var lounge = "navy"
    /// The lounge's wall: "auto", as the office's walls, or a paint of its own.
    var loungePaint = "auto"
    var corner = "arcade"
    var sky = "live"
    /// The office's pets, as many as `petsMost`, each once, in the order
    /// they were picked. In a style, none picked is the style's own.
    var pet: [String] = []
    var season = "calendar"
    var sound = "off"
    var desk = "wood"
    var workspace = "desk"
    var monitor = "standard"
    var chair = "teal"
    var staff = "some"
    var preset = "yours"
    /// How each project's agent looks, by the project's name.
    var team: [String: TeamMember] = [:]
    /// What agents wear by their model's size and by agent. After the team,
    /// in the same place as the Windows and Linux app's Room has it.
    var dress = Dress()
    /// Where each piece of the room is, as arranged on the setup page.
    var plan = Plan()

    /// The most projects a team remembers.
    static let teamLimit = 50

    /// Each part's choices; the first is the standard one.
    static let choices = [
        "mic": ["mic", "megaphone", "studio", "podcast", "karaoke", "headset", "walkie", "tincan", "radio"],
        "floor": Room.floors,
        "paint": Room.paints,
        // The lounge's carpets, or any of the office's floors
        "lounge": ["navy", "teal", "plum", "sand"] + Room.floors,
        "loungePaint": ["auto"] + Room.paints,
        "corner": ["arcade", "vending", "foosball", "fishtank", "books", "dragon", "pinball", "jukebox", "pool", "piano", "drums",
                   "telescope", "jellyfish", "bonsai", "jungle", "coffee", "servers", "printer", "boxing", "treadmill", "hottub",
                   "ballpit", "campfire", "bubbler", "neonbox", "wallbox", "djtable", "turntable", "boombox", "hifi", "synth", "none"],
        "sky": ["live", "day"],
        "season": ["calendar", "off"],
        "sound": ["off", "quiet", "normal"],
        "desk": ["wood", "walnut", "white", "black"],
        "workspace": Room.workspaces,
        "monitor": ["standard", "crt", "holo", "brass", "crystal", "gilt", "ice", "ultrawide", "dual", "laptop", "bamboo", "tablet",
                    "portrait"],
        "chair": ["teal", "black", "red", "grey", "green"],
        "staff": ["some", "none", "many"],
        "preset": Room.presets,
    ]
    static let floors = ["wood", "tiles", "concrete", "carpet", "plating", "deck", "marble", "flagstone", "snow", "grass", "forest",
                         "rubber", "epoxy", "arcade"]
    static let paints = ["slate", "sage", "clay", "cream", "hull", "timber", "damask", "stone", "ice", "logs", "brick", "bamboo",
                         "block", "foam"]
    /// The office's styles: Yours, as you furnished it, then each that
    /// furnishes it for you. office.js keeps what each one is.
    static let presets = ["yours", "classic", "campsite", "royal", "space", "pirates", "arctic", "lair", "gamers", "garage", "gym", "jungle"]
    /// What an agent can work at: the office's choice for everyone, and a
    /// team member's own, which can also be "auto".
    static let workspaces = ["desk", "standing", "gaming", "tent", "box", "beanbag", "igloo", "van", "cockpit", "ship", "throne",
                             "greenhouse", "console", "hover", "chest", "gilded", "crates", "altar", "log", "bench", "bamboo", "rack",
                             "drafting", "glass"]
    static let wallChoices = ["window", "whiteboard", "pinboard", "poster", "shelf", "plant", "bare",
                              "viewport", "porthole", "panel", "pipes", "seaPorthole", "map", "flag", "cutlasses", "tapestry",
                              "portrait", "stained", "shield", "frost", "skis", "weather", "torch", "arch", "neon", "tv", "hex",
                              "pegboard", "roller", "bike", "mirror", "timer", "climbing", "vines", "tiki", "blind", "cabin", "trail",
                              "paddles"]
    static let standardWall = ["window", "whiteboard", "window", "pinboard", "window"]
    /// The pets an office can have, and the most it has at once.
    static let pets = ["cat", "dog", "lizard", "monkey", "llama", "penguin", "parrot", "owl", "capybara", "redpanda", "corgi", "hamster",
                       "robodog", "bat", "snail", "fox", "axolotl"]
    static let petsMost = 3

    init() {}

    init(from decoder: Decoder) throws {
        let c = try? decoder.container(keyedBy: CodingKeys.self)
        func choice(_ key: CodingKeys) -> String? {
            let given = (try? c?.decodeIfPresent(String.self, forKey: key)) ?? nil
            return given.flatMap { Room.choices[key.stringValue]?.contains($0) == true ? $0 : nil }
        }
        mic = choice(.mic) ?? mic
        floor = choice(.floor) ?? floor
        paint = choice(.paint) ?? paint
        lounge = choice(.lounge) ?? lounge
        loungePaint = choice(.loungePaint) ?? loungePaint
        corner = choice(.corner) ?? corner
        sky = choice(.sky) ?? sky
        // The pets as a list, or one by its id, as an office had before it could have more
        let pets = ((try? c?.decodeIfPresent([Loose].self, forKey: .pet)) ?? nil)?.map(\.text)
            ?? [(try? c?.decodeIfPresent(String.self, forKey: .pet)) ?? nil]
        pet = Room.checkedPets(pets.compactMap { $0 })
        season = choice(.season) ?? season
        sound = choice(.sound) ?? sound
        desk = choice(.desk) ?? desk
        workspace = choice(.workspace) ?? workspace
        monitor = choice(.monitor) ?? monitor
        chair = choice(.chair) ?? chair
        staff = choice(.staff) ?? staff
        preset = choice(.preset) ?? preset
        let given = ((try? c?.decodeIfPresent([Loose].self, forKey: .wall)) ?? nil) ?? []
        wall = Room.standardWall.enumerated().map { i, standard in
            i < given.count ? given[i].text.flatMap { Room.wallChoices.contains($0) ? $0 : nil } ?? standard : standard
        }
        let members = ((try? c?.decodeIfPresent([String: TeamMember].self, forKey: .team)) ?? nil) ?? [:]
        team = Dictionary(uniqueKeysWithValues: members.filter { !$0.key.isEmpty }.sorted { $0.key < $1.key }
            .prefix(Room.teamLimit).map { ($0.key, $0.value) })
        dress = ((try? c?.decodeIfPresent(Dress.self, forKey: .dress)) ?? nil) ?? Dress()
        plan = ((try? c?.decodeIfPresent(Plan.self, forKey: .plan)) ?? nil) ?? Plan()
    }

    /// The pets given that an office can have, each once and as many as it
    /// has at once, in their order.
    static func checkedPets(_ given: [String]) -> [String] {
        var checked: [String] = []
        for pet in given where Room.pets.contains(pet) && !checked.contains(pet) && checked.count < Room.petsMost {
            checked.append(pet)
        }
        return checked
    }

    /// One of the wall's places as given: text, or nothing for any other kind of value.
    private struct Loose: Decodable {
        let text: String?
        init(from decoder: Decoder) throws { text = try? decoder.singleValueContainer().decode(String.self) }
    }
}

/// How one project's agent looks in the office: skin, hair, a hairstyle,
/// something extra such as glasses or a cap, an outfit and its colour, what's
/// on its desk, what it works at, and a nickname for it. "auto" is as the
/// project's name picks it, the clothes as its model's, and what it works at
/// as everyone does. office.js draws it, with the same choices.
struct TeamMember: Codable, Equatable {
    var skin = "auto"
    var hair = "auto"
    var hairstyle = "auto"
    var extra = "none"
    var outfit = "auto"
    var colour = "auto"
    var desk = "auto"
    var workspace = "auto"
    var name = ""

    /// Each part's choices; the first is the standard one.
    static let choices = [
        "skin": ["auto", "porcelain", "fair", "light", "tan", "olive", "brown", "deep", "ebony", "cream"],
        "hair": ["auto", "black", "espresso", "brown", "chestnut", "auburn", "ginger", "blond", "platinum", "grey", "white",
                 "pink", "blue", "green", "purple", "orange"],
        "hairstyle": ["auto", "short", "long", "spiky", "bun", "curly", "ponytail", "afro", "mohawk", "buzz", "bald"],
        "extra": ["none", "glasses", "sunglasses", "headphones", "cap", "beanie", "beard", "moustache", "bow"],
        "outfit": ["auto", "tee", "hoodie", "collar", "sweater", "jacket", "suit", "labcoat", "polo", "cardigan", "hawaiian",
                   "tracksuit", "overalls", "waistcoat",
                   // A style's own, and each model family's
                   "astronaut", "flightsuit", "pirate", "captain", "parka", "explorer", "robe", "knight", "wizard", "gamer",
                   "founder", "gymkit", "safari", "flannel", "hire", "claude", "gpt", "gemini", "grok", "qwen", "llama", "mistral",
                   "deepseek", "kimi", "glm", "phi"],
        "colour": ["auto", "orange", "red", "pink", "purple", "blue", "teal", "green", "yellow", "grey", "black", "white"],
        "desk": ["auto", "plant", "papers", "lamp", "cactus", "duck", "trophy", "photo", "mug",
                 "helmet", "rocket", "terrarium", "quill", "candelabra", "goblet", "rum", "compass", "skull", "snowglobe", "thermos",
                 "penguin", "potions", "egg", "spellbook", "controller", "energy", "headset", "circuit", "pizza", "coffeepot",
                 "dumbbell", "shaker", "kettlebell", "coconut", "pineapple", "tikimug", "enamel", "lantern", "pinecone", "none"],
        "workspace": ["auto"] + Room.workspaces,
    ]
    /// The most a nickname can be, in characters.
    static let nicknameLimit = 16

    init(from decoder: Decoder) throws {
        let c = try? decoder.container(keyedBy: CodingKeys.self)
        func choice(_ key: CodingKeys) -> String? {
            let given = (try? c?.decodeIfPresent(String.self, forKey: key)) ?? nil
            return given.flatMap { TeamMember.choices[key.stringValue]?.contains($0) == true ? $0 : nil }
        }
        skin = choice(.skin) ?? skin
        hair = choice(.hair) ?? hair
        hairstyle = choice(.hairstyle) ?? hairstyle
        extra = choice(.extra) ?? extra
        outfit = choice(.outfit) ?? outfit
        colour = choice(.colour) ?? colour
        desk = choice(.desk) ?? desk
        workspace = choice(.workspace) ?? workspace
        name = (((try? c?.decodeIfPresent(String.self, forKey: .name)) ?? nil) ?? "").oneLine(TeamMember.nicknameLimit)
    }
}

/// The office's dress code: what agents wear by the size of their model, and
/// by which agent they are, and rules that give a model its size by part of
/// its name. Every size and every agent is always there, "auto" when it isn't
/// set, so office.js never finds one missing. Anything unknown, or of the
/// wrong kind, is as standard, as in the rest of the room. The Windows and
/// Linux app's Dress matches.
/// The room arranged your way, from the setup page's editor: for each shape
/// of the room, `wide` and `tall`, the pieces moved from where they stand,
/// made bigger, or put away, and the pieces added to the lounge. office.js
/// draws it, with the same pieces and sizes. Anything unknown is left out,
/// so the piece stands as it comes.
struct Plan: Codable, Equatable {
    var wide: [String: Piece] = [:]
    var tall: [String: Piece] = [:]
    var added = Added()
    var hung = Hung()

    /// What hangs on the back wall, for each shape of the room: each piece
    /// round its middle, where you put it. nil keeps the wall's five places,
    /// as set up; an empty list is a bare wall.
    struct Hung: Codable, Equatable {
        var wide: [HungPiece]?
        var tall: [HungPiece]?

        init() {}

        init(from decoder: Decoder) throws {
            let c = try? decoder.container(keyedBy: CodingKeys.self)
            func pieces(_ key: CodingKeys) -> [HungPiece]? {
                guard let given = (try? c?.decodeIfPresent([LooseHung].self, forKey: key)) ?? nil else { return nil }
                return Array(given.compactMap(\.piece).prefix(Plan.hungMost))
            }
            wide = pieces(.wide)
            tall = pieces(.tall)
        }
    }

    struct HungPiece: Codable, Equatable {
        var kind: String
        var x = 0
    }

    /// The most pieces the back wall holds, in each shape.
    static let hungMost = 12
    /// How far along the wall a piece can be, in the room's pixels: the wide room's width.
    static let wallLength = 320

    /// Pieces added to the lounge, for each shape of the room: any its
    /// corner can hold, the same one more than once, each moved from where
    /// the corner's piece stands, and grown.
    struct Added: Codable, Equatable {
        var wide: [AddedPiece] = []
        var tall: [AddedPiece] = []

        init() {}

        init(from decoder: Decoder) throws {
            let c = try? decoder.container(keyedBy: CodingKeys.self)
            func pieces(_ key: CodingKeys) -> [AddedPiece] {
                let given = ((try? c?.decodeIfPresent([LooseAdded].self, forKey: key)) ?? nil) ?? []
                return Array(given.compactMap(\.piece).prefix(Plan.addedMost))
            }
            wide = pieces(.wide)
            tall = pieces(.tall)
        }
    }

    struct AddedPiece: Codable, Equatable {
        var kind: String
        var x = 0
        var y = 0
        var size = 1.0
    }

    /// The most pieces that can be added to the lounge, in each shape.
    static let addedMost = 12
    /// What can be added: anything the lounge's corner can hold.
    static var addable: [String] { (Room.choices["corner"] ?? []).filter { $0 != "none" } }

    struct Piece: Codable, Equatable {
        var x = 0
        var y = 0
        var size = 1.0
        var off = false
        /// A desk's own look, whoever sits at it: its workspace and its
        /// monitor, finish, chair and thing, unless their project has its
        /// own workspace or thing; "auto" for each otherwise, and on what
        /// isn't a desk.
        var workspace = "auto"
        var monitor = "auto"
        var finish = "auto"
        var chair = "auto"
        var thing = "auto"
    }

    /// What can be arranged: each desk, which moves and can be put away,
    /// and the lounge's pieces, which can grow, and be put away but for the
    /// red button and the mic, which are controls.
    static let pieces: [String: (grows: Bool, hides: Bool)] = {
        var pieces = Dictionary(uniqueKeysWithValues: (1...9).map { ("desk\($0)", (grows: false, hides: true)) })
        for part in ["wall", "corner", "couch", "pong", "plant"] { pieces[part] = (true, true) }
        for part in ["button", "mic"] { pieces[part] = (true, false) }
        return pieces
    }()
    static let sizes: [Double] = [1, 1.5, 2]
    /// How far a piece can be moved, either way, in the room's pixels.
    static let reach = 400

    init() {}

    init(from decoder: Decoder) throws {
        let c = try? decoder.container(keyedBy: CodingKeys.self)
        func arranged(_ key: CodingKeys) -> [String: Piece] {
            let given = ((try? c?.decodeIfPresent([String: Loose].self, forKey: key)) ?? nil) ?? [:]
            return given.reduce(into: [:]) { pieces, entry in
                guard let kind = Plan.pieces[entry.key], let piece = entry.value.piece else { return }
                let at = { (n: Double?) in Plan.whole(n, within: -Plan.reach...Plan.reach) }
                let desk = entry.key.hasPrefix("desk")
                // One of a desk's own, or "auto"
                func own(_ name: String, _ choices: [String]) -> String {
                    guard desk, let given = piece.texts[name], choices.contains(given) else { return "auto" }
                    return given
                }
                pieces[entry.key] = Piece(x: at(piece.x), y: at(piece.y),
                                          size: kind.grows && Plan.sizes.contains(piece.size ?? 1) ? piece.size ?? 1 : 1,
                                          off: kind.hides && piece.off == true,
                                          workspace: own("workspace", Room.workspaces),
                                          monitor: own("monitor", Room.choices["monitor"] ?? []),
                                          finish: own("finish", Room.choices["desk"] ?? []),
                                          chair: own("chair", Room.choices["chair"] ?? []),
                                          thing: own("thing", (TeamMember.choices["desk"] ?? []).filter { $0 != "auto" }))
            }
        }
        wide = arranged(.wide)
        tall = arranged(.tall)
        added = ((try? c?.decodeIfPresent(Added.self, forKey: .added)) ?? nil) ?? Added()
        hung = ((try? c?.decodeIfPresent(Hung.self, forKey: .hung)) ?? nil) ?? Hung()
    }

    /// A piece on the back wall as given, read leniently: one that isn't anything the wall can hold is left out.
    private struct LooseHung: Decodable {
        let piece: HungPiece?
        private enum Keys: String, CodingKey { case kind, x }
        init(from decoder: Decoder) throws {
            guard let c = try? decoder.container(keyedBy: Keys.self),
                  let kind = (try? c.decodeIfPresent(String.self, forKey: .kind)) ?? nil, Room.wallChoices.contains(kind), kind != "bare" else {
                piece = nil
                return
            }
            let x = ((try? c.decodeIfPresent(Double.self, forKey: .x)) ?? nil) ?? 110
            piece = HungPiece(kind: kind, x: Plan.whole(x, within: 0...Plan.wallLength))
        }
    }

    /// Where a piece is moved to, within reach.
    fileprivate static func at(_ n: Double?) -> Int { whole(n, within: -reach...reach) }

    /// `n` as a whole number within `range`: clamped before it's made whole,
    /// as a number too big for an Int would stop the app, and 0 when it isn't
    /// a number at all.
    fileprivate static func whole(_ n: Double?, within range: ClosedRange<Int>) -> Int {
        guard let n, n.isFinite else { return min(max(0, range.lowerBound), range.upperBound) }
        return Int(min(Double(range.upperBound), max(Double(range.lowerBound), n)).rounded())
    }

    /// An added piece as given, read leniently: one that isn't anything the corner can hold is left out.
    private struct LooseAdded: Decodable {
        let piece: AddedPiece?
        private enum Keys: String, CodingKey { case kind, x, y, size }
        init(from decoder: Decoder) throws {
            guard let c = try? decoder.container(keyedBy: Keys.self),
                  let kind = (try? c.decodeIfPresent(String.self, forKey: .kind)) ?? nil, Plan.addable.contains(kind) else {
                piece = nil
                return
            }
            func number(_ key: Keys) -> Double? { (try? c.decodeIfPresent(Double.self, forKey: key)) ?? nil }
            let size = number(.size) ?? 1
            piece = AddedPiece(kind: kind, x: Plan.at(number(.x)), y: Plan.at(number(.y)), size: Plan.sizes.contains(size) ? size : 1)
        }
    }

    /// A piece as given, each of its parts read on its own: one of the wrong
    /// kind is as standard.
    private struct Loose: Decodable {
        let piece: (x: Double?, y: Double?, size: Double?, off: Bool?, texts: [String: String])?
        private enum Keys: String, CodingKey { case x, y, size, off, workspace, monitor, finish, chair, thing }
        init(from decoder: Decoder) throws {
            guard let c = try? decoder.container(keyedBy: Keys.self) else {
                piece = nil
                return
            }
            func number(_ key: Keys) -> Double? { (try? c.decodeIfPresent(Double.self, forKey: key)) ?? nil }
            var texts: [String: String] = [:]
            for key in [Keys.workspace, .monitor, .finish, .chair, .thing] {
                if let text = (try? c.decodeIfPresent(String.self, forKey: key)) ?? nil { texts[key.stringValue] = text }
            }
            piece = (number(.x), number(.y), number(.size), (try? c.decodeIfPresent(Bool.self, forKey: .off)) ?? nil, texts)
        }
    }
}

struct Dress: Codable, Equatable {
    /// By the size of the model: small, medium, large and top.
    var sizes = Dress.standard(Dress.sizeNames)
    /// By agent: one for each the deck follows.
    var agents = Dress.standard(Dress.agentNames)
    /// In the order given.
    var rules: [Rule] = []

    static let sizeNames = ["small", "medium", "large", "top"]
    static let agentNames = Agent.allCases.map(\.rawValue)
    /// The most rules kept, and the most a rule's part of a name can be, in
    /// characters.
    static let ruleLimit = 20
    static let containsLimit = 32

    init() {}

    init(from decoder: Decoder) throws {
        let c = try? decoder.container(keyedBy: CodingKeys.self)
        func attires(_ key: CodingKeys, _ names: [String]) -> [String: Attire] {
            let given = ((try? c?.decodeIfPresent([String: Attire].self, forKey: key)) ?? nil) ?? [:]
            return Dictionary(uniqueKeysWithValues: names.map { ($0, given[$0] ?? Attire()) })
        }
        sizes = attires(.sizes, Dress.sizeNames)
        agents = attires(.agents, Dress.agentNames)
        // A rule with nothing to look for, or a size the office doesn't have, is dropped.
        let given = ((try? c?.decodeIfPresent([Rule].self, forKey: .rules)) ?? nil) ?? []
        rules = Array(given.filter { !$0.contains.isEmpty && Dress.sizeNames.contains($0.size) }.prefix(Dress.ruleLimit))
    }

    private static func standard(_ names: [String]) -> [String: Attire] {
        Dictionary(uniqueKeysWithValues: names.map { ($0, Attire()) })
    }

    /// A model whose name has `contains` in it is dressed as `size`. What it
    /// looks for is kept on one line, and lowercased.
    struct Rule: Codable, Equatable {
        var contains = ""
        var size = ""

        init(from decoder: Decoder) throws {
            let c = try? decoder.container(keyedBy: CodingKeys.self)
            func text(_ key: CodingKeys) -> String { ((try? c?.decodeIfPresent(String.self, forKey: key)) ?? nil) ?? "" }
            contains = text(.contains).lowercased().oneLine(Dress.containsLimit)
            size = text(.size)
        }
    }
}

/// What the dress code puts on agents: an outfit and its colour, from a team
/// member's choices. "auto" is as its model's, as for a team member.
struct Attire: Codable, Equatable {
    var outfit = "auto"
    var colour = "auto"

    init() {}

    init(from decoder: Decoder) throws {
        let c = try? decoder.container(keyedBy: CodingKeys.self)
        func choice(_ key: CodingKeys) -> String? {
            let given = (try? c?.decodeIfPresent(String.self, forKey: key)) ?? nil
            return given.flatMap { TeamMember.choices[key.stringValue]?.contains($0) == true ? $0 : nil }
        }
        outfit = choice(.outfit) ?? outfit
        colour = choice(.colour) ?? colour
    }
}

/// How many session keys the deck shows: Auto, one for each running session,
/// or always 3, 6 or 9. In JSON, Auto is "auto".
enum KeyCount: Codable, Equatable {
    case auto
    case fixed(Int)

    /// The most keys there can be, in a 3×3.
    static let most = 9
    static let fixedCounts = [3, 6, 9]

    /// Anything but a fixed count reads as Auto.
    init(from decoder: Decoder) throws {
        let count = try? decoder.singleValueContainer().decode(Int.self)
        self = count.map { KeyCount.fixedCounts.contains($0) ? .fixed($0) : .auto } ?? .auto
    }

    func encode(to encoder: Encoder) throws {
        var c = encoder.singleValueContainer()
        switch self {
        case .auto: try c.encode("auto")
        case .fixed(let count): try c.encode(count)
        }
    }
}

/// A row of buttons, tall or short, sharing its width by weight.
struct ButtonRow: Codable, Equatable {
    var tall = false
    var buttons: [Button] = []

    init(tall: Bool, buttons: [Button]) {
        self.tall = tall
        self.buttons = buttons
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        tall = try c.decodeIfPresent(Bool.self, forKey: .tall) ?? false
        buttons = try c.decodeIfPresent([Button].self, forKey: .buttons) ?? []
    }
}

/// One of the deck's own buttons, named by what it does, or one made in
/// setup, which types a command or presses a key. The phone names a made
/// button only by its id, so it can't have anything typed that wasn't set
/// up here.
struct Button: Codable, Equatable {
    var id: String
    var width = 1
    var label: String?
    var text: String?
    var key: String?
    /// Refused while the agent is busy, as /clear is.
    var whenFree: Bool?
    /// Asks for a second tap, as /clear does.
    var confirm: Bool?
    /// The agents it is for; none means every agent.
    var agents: [String]?

    static let maxWidth = 3
    static let labelLimit = 16
    static let commandLimit = 200

    /// The deck's own buttons. Max and the steps set Claude Code's effort,
    /// as the dial does.
    static let builtIn: Set = ["accept", "reject", "mic", "esc", "send", "clear", "up", "down", "max", "efforts"]

    /// The keys a made button can press.
    static let keys: [String: Terminal.Keystroke] = [
        "enter": .init(key: .returnKey), "escape": .init(key: .escape), "tab": .init(key: .tab),
        "shift-tab": .init(key: .tab, modifiers: .maskShift), "up": .init(key: .up), "down": .init(key: .down),
        "ctrl-c": .init(key: .c, modifiers: .maskControl),
    ]

    init(id: String, width: Int = 1) {
        self.id = id
        self.width = width
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decode(String.self, forKey: .id)
        width = try c.decodeIfPresent(Int.self, forKey: .width) ?? 1
        label = try c.decodeIfPresent(String.self, forKey: .label)
        text = try c.decodeIfPresent(String.self, forKey: .text)
        // An empty key is none at all: the button types its text.
        key = try c.decodeIfPresent(String.self, forKey: .key).flatMap { $0.isEmpty ? nil : $0 }
        whenFree = try c.decodeIfPresent(Bool.self, forKey: .whenFree)
        confirm = try c.decodeIfPresent(Bool.self, forKey: .confirm)
        agents = try c.decodeIfPresent([String].self, forKey: .agents)
    }

    var checked: Button? {
        let width = (1...Button.maxWidth).contains(width) ? width : 1
        if Button.builtIn.contains(id) { return Button(id: id, width: width) }
        guard id.range(of: "^custom-[a-z0-9]{1,16}$", options: .regularExpression) != nil else { return nil }
        // A command with a line break, or a key the deck can't press, is
        // refused whole: the editor can't make one, so it came from somewhere else.
        if let key, Button.keys[key] == nil { return nil }
        if let text, text.hasControlCharacters { return nil }

        var checked = Button(id: id, width: width)
        checked.label = label.map { $0.oneLine(Button.labelLimit) }.flatMap { $0.isEmpty ? nil : $0 }
        if let key { checked.key = key } else { checked.text = (text ?? "").oneLine(Button.commandLimit) }
        checked.whenFree = whenFree == true ? true : nil
        checked.confirm = confirm == true ? true : nil
        let known = Agent.allCases.map(\.rawValue).filter { (agents ?? []).contains($0) }
        checked.agents = known.isEmpty || known.count == Agent.allCases.count ? nil : known
        return checked
    }
}

/// A look for the deck kept under a name: a layout and its colours. Default
/// is always there, the standard deck, and can't be changed.
struct Profile: Codable, Equatable {
    var name: String
    var layout: Layout
    var colours: [String: String]

    static let defaultName = "Default"
    static let nameLimit = 24
    static let most = 20

    static let standard = Profile(name: defaultName, layout: .standard, colours: [:])

    /// A name a profile may have: one line, not empty, and not Default's.
    static func checkedName(_ name: String) -> String? {
        let name = name.oneLine(nameLimit)
        return name.isEmpty || name.caseInsensitiveCompare(defaultName) == .orderedSame ? nil : name
    }

    var jsonObject: Any {
        (try? JSONSerialization.jsonObject(with: JSONEncoder().encode(self))) ?? [:]
    }
}

extension String {
    var hasControlCharacters: Bool {
        unicodeScalars.contains { $0.value < 0x20 || $0.value == 0x7f }
    }

    /// Without control characters or surrounding space, cut to `limit` characters.
    func oneLine(_ limit: Int) -> String {
        let kept = String(String.UnicodeScalarView(unicodeScalars.filter { $0.value >= 0x20 && $0.value != 0x7f }))
        return String(kept.trimmingCharacters(in: .whitespaces).prefix(limit))
    }
}
