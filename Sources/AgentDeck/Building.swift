import Foundation

// MARK: Who sits on which floor

/// A desk above the ground floor: its floor, 1 for 2F up to 5 for 6F, and
/// which of that floor's desks it is.
struct Seat: Codable, Equatable, Hashable {
    let floor: Int
    let desk: Int
}

/// A folder whose sessions sit on a floor of their own: "Atlas projects go
/// to the Atlas floor". `floor` is 0 for the ground floor up to 5 for 6F.
struct FloorRule: Codable, Equatable {
    var folder: String
    var floor: Int
    /// The folder alone, not those in it: as "Always put … here" remembers
    /// it from the office, where a folder above a project's would otherwise
    /// take all of them up. A rule added in setup holds the folders in it too.
    var exact = false

    static let most = 64

    init(folder: String, floor: Int, exact: Bool = false) {
        self.folder = folder
        self.floor = floor
        self.exact = exact
    }

    /// Read leniently: a rule from before `exact` holds the folders in it.
    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        folder = try c.decode(String.self, forKey: .folder)
        floor = try c.decode(Int.self, forKey: .floor)
        exact = (try? c.decodeIfPresent(Bool.self, forKey: .exact)) ?? false
    }

    /// As they're kept: each folder a path, with no slash at its end, on a
    /// floor there is; a folder's rule for itself alone, and its rule for
    /// the folders in it too, one of each, the last given winning.
    static func checked(_ given: [FloorRule]) -> [FloorRule] {
        var rules: [FloorRule] = []
        for rule in given {
            guard let folder = ruleFolder(rule.folder), (0..<Floor.count).contains(rule.floor) else { continue }
            // One of each kind a folder: "Always put … here" for itself alone beside setup's for the folders in it.
            rules.removeAll { $0.folder == folder && $0.exact == rule.exact }
            rules.append(FloorRule(folder: folder, floor: rule.floor, exact: rule.exact))
        }
        return Array(rules.suffix(most))
    }

    /// A rule's folder as it's kept: a whole path, a Mac's (/…), under your
    /// home (~/…), or a Windows computer's (C:\…, \\server\…), kept as given for
    /// a config that goes back there; with no separator at its end. Nil for
    /// one that isn't a whole path.
    static func ruleFolder(_ given: String) -> String? {
        var folder = given.trimmingCharacters(in: .whitespacesAndNewlines)
        if folder == "~" || folder.hasPrefix("~/") { folder = (folder as NSString).expandingTildeInPath }
        let chars = Array(folder)
        let windows = chars.count >= 3 && chars[0].isASCII && chars[0].isLetter && chars[1] == ":" && (chars[2] == "\\" || chars[2] == "/")
        guard folder.hasPrefix("/") || folder.hasPrefix("\\\\") || windows else { return nil }
        while folder.count > 1, let last = folder.last, last == "/" || last == "\\", !(windows && folder.count == 3) { folder.removeLast() }
        return folder
    }

    /// The floor `project` goes to by its rules: the rule for the folder it's
    /// in, or the closest above it. Nil when no rule says.
    static func floor(of project: String, in rules: [FloorRule]) -> Int? {
        guard !project.isEmpty else { return nil }
        // The closest folder; on that folder itself, its own rule before the one for what's in it; then the later.
        return rules.enumerated()
            .filter { _, rule in project == rule.folder || !rule.exact && project.hasPrefix(rule.folder == "/" ? "/" : rule.folder + "/") }
            .max { a, b in (a.element.folder.count, a.element.exact ? 1 : 0, a.offset) < (b.element.folder.count, b.element.exact ? 1 : 0, b.offset) }?
            .element.floor
    }

    /// The folder a hire on `floor` starts in: the floor's one rule's, or
    /// nil when it has none or more than one.
    static func folder(of floor: Int, in rules: [FloorRule]) -> String? {
        let on = Set(rules.filter { $0.floor == floor }.map(\.folder))
        return on.count == 1 ? on.first : nil
    }

    var jsonObject: [String: Any] { ["folder": folder, "floor": floor, "exact": exact] }
}

/// Someone to seat above the ground floor: `id` to know them by, the floor
/// they were put on, if they were (1 to 5), and the desk they had, to keep it.
struct Upstairs<Person> {
    let person: Person
    let id: String
    var floor: Int? = nil
    var had: Seat? = nil
    /// A manager's hire: new on a floor, it takes a desk by whole rows (see `Deck.rowDesk`).
    var byRow = false
    /// On its floor because nobody put it there, as a cloud bot that goes where its folder's rule or new bots
    /// go: on that floor full, it takes nobody's desk, and sits where there's room, keeping that desk as others
    /// come and go till one is free on its own floor. There only for want of room, it makes way for anyone put on
    /// that floor (see `Deck.seat`).
    var yields = false
}

extension Deck {
    /// How many desks make a row on a floor above: the office draws three to a row, as downstairs.
    static let deskRow = 3

    /// The desk a manager's hire takes on a floor whose desks `taken` are, so a team reads as a row: the first
    /// free desk, left to right, of the last row anyone sits in; when that row is full, the first desk of the
    /// next. Gaps in earlier rows stay empty, since nobody moves. With no row left whole, the first free desk.
    static func rowDesk(taken: Set<Int>, row: Int = Deck.deskRow, desks: Int = Floor.desks) -> Int? {
        let free = { (desk: Int) in !taken.contains(desk) }
        if let last = taken.filter({ $0 < desks }).max().map({ $0 / row }) {
            if let desk = (last * row..<min((last + 1) * row, desks)).first(where: free) { return desk }
            if (last + 1) * row < desks { return (last + 1) * row }
        }
        return (0..<desks).first(where: free)
    }

    /// The building above the ground floor, `Floor.desks` desks a floor.
    /// Those put on a floor sit there first, at the desk they had if it's
    /// still free, else at its first free one; a floor that's full sends the
    /// rest to be seated with everyone else, past anyone there only for want
    /// of room elsewhere unless they `yields`. One who yields, sent on from
    /// its floor, keeps the desk it has on another as anyone not put on a
    /// floor does, making way for those put there, till its own has a desk
    /// for it. Then everyone else, in order: at the desk they had if
    /// it's free, else at the first free desk from 2F up. So nobody moves
    /// when someone else comes or goes. Each floor's people in the order of
    /// their desks, with them; how many there's no desk for; and the desk
    /// each one has now.
    static func seat<Person>(placed: [Upstairs<Person>], rest: [Upstairs<Person>])
        -> (floors: [[(person: Person, desk: Int)]], overflow: Int, seats: [String: Seat]) {
        var taken: [Seat: Int] = [:]  // the desk, and who's at it, by their place in `everyone`
        var seats: [String: Seat] = [:]
        let everyone = placed + rest
        let isFree = { (seat: Seat) in taken[seat] == nil }
        func sit(_ index: Int, at seat: Seat) {
            taken[seat] = index
            seats[everyone[index].id] = seat
        }
        let floors = 1..<Floor.count
        let ok = { (seat: Seat?) -> Seat? in
            guard let seat, floors.contains(seat.floor), (0..<Floor.desks).contains(seat.desk) else { return nil }
            return seat
        }
        // First everyone at the desk they had, so nobody moves for someone new: those put on a floor at theirs there,
        // then everyone else at theirs, wherever it is; then those who yield at theirs on a floor not their own, where
        // theirs had no room: after the rest, so one back at a desk it had takes no one's.
        var unseated: [Int] = []
        for (index, one) in placed.enumerated() {
            guard let floor = one.floor, floors.contains(floor) else { unseated.append(index); continue }
            if let had = ok(one.had), had.floor == floor, isFree(had) { sit(index, at: had) }
        }
        let placedCount = placed.count
        for index in rest.indices.map({ placedCount + $0 }) {
            if let had = ok(everyone[index].had), isFree(had) { sit(index, at: had) }
        }
        for (index, one) in placed.enumerated() where one.yields && seats[one.id] == nil && !unseated.contains(index) {
            if let had = ok(one.had), isFree(had) { sit(index, at: had) }
        }
        // On `floor` only for want of room on its own: one who yields; and anyone not put on a floor, there for want
        // of room anywhere.
        let away = { (index: Int, floor: Int) in index < placedCount && everyone[index].yields && everyone[index].floor != floor }
        let makesWay = { (index: Int, floor: Int) in index >= placedCount || away(index, floor) }
        let desks = { (floor: Int) in (0..<Floor.desks).map { Seat(floor: floor, desk: $0) } }
        // A free desk on `floor` for `one`: a manager's hire's by whole rows, anyone else's the first.
        // A desk kept for someone put on another floor and sent on from it, who sits again at the desk they had
        // below: one who yields takes nobody's desk, theirs included.
        let keptFor = { (seat: Seat) in
            placed.indices.contains { !placed[$0].yields && seats[placed[$0].id] == nil && ok(placed[$0].had) == seat }
        }
        let open = { (one: Upstairs<Person>, seat: Seat) in isFree(seat) && !(one.yields && keptFor(seat)) }
        let freeDesk = { (one: Upstairs<Person>, floor: Int) -> Seat? in
            guard one.byRow else { return desks(floor).first { open(one, $0) } }
            return Deck.rowDesk(taken: Set(desks(floor).filter { !open(one, $0) }.map(\.desk))).map { Seat(floor: floor, desk: $0) }
        }
        // Those put on a floor and new there: at a free desk on it; on a floor that's full, at the desk of someone
        // there only for want of room elsewhere, who goes to the floors with room, unless nobody put them there
        // (`yields`). With neither, where there's room.
        for (index, one) in placed.enumerated() where seats[one.id] == nil && !unseated.contains(index) {
            guard let floor = one.floor else { continue }
            if let free = freeDesk(one, floor) {
                sit(index, at: free)
            } else if !one.yields, let seat = desks(floor).first(where: { taken[$0].map { makesWay($0, floor) } ?? false }),
                      let other = taken[seat] {
                seats[everyone[other].id] = nil  // seated again below, with everyone not seated yet
                if other < placedCount { unseated.append(other) }
                sit(index, at: seat)
            } else {
                unseated.append(index)
            }
        }
        // Then those who yield and aren't on their floor, new or sent on from it: at a free desk there; or, two each on
        // the other's, at each other's desks. Again till nobody moves, as one going home can free a desk for another.
        var moved = true
        while moved {
            moved = false
            for (index, one) in placed.enumerated() where one.yields {
                guard let floor = one.floor, floors.contains(floor), seats[one.id]?.floor != floor else { continue }
                let was = seats[one.id]
                if let free = freeDesk(one, floor) {
                    if let was { taken[was] = nil }
                    sit(index, at: free)
                    moved = true
                } else if let was, let seat = desks(floor).first(where: { taken[$0].map { away($0, floor) && everyone[$0].floor == was.floor } ?? false }),
                          let other = taken[seat] {
                    sit(index, at: seat)
                    sit(other, at: was)
                    moved = true
                }
            }
        }
        // Then everyone else not seated yet, and those whose floor was full: at the desk they had if it's still free
        // (so nobody sent on from a full floor shifts when a bot or anyone else arrives), else at the first free desk from 2F up.
        var left = (unseated + rest.indices.map { placedCount + $0 }).filter { seats[everyone[$0].id] == nil }
        for index in left { if let had = ok(everyone[index].had), isFree(had) { sit(index, at: had) } }
        left = left.filter { seats[everyone[$0].id] == nil }
        var overflow = 0
        let all = floors.flatMap { floor in (0..<Floor.desks).map { Seat(floor: floor, desk: $0) } }
        for index in left {
            var free = all.first(where: isFree)
            if everyone[index].byRow {
                // A manager's hire sits by rows on the first floor with room.
                free = floors.lazy.compactMap { floor in
                    Deck.rowDesk(taken: Set((0..<Floor.desks).filter { !isFree(Seat(floor: floor, desk: $0)) })).map { Seat(floor: floor, desk: $0) }
                }.first
            }
            guard let free else { overflow += 1; continue }
            sit(index, at: free)
        }
        let shown = (0..<Floor.count).map { floor -> [(person: Person, desk: Int)] in
            (0..<Floor.desks).compactMap { desk in taken[Seat(floor: floor, desk: desk)].map { (everyone[$0].person, desk) } }
        }
        return (shown, overflow, seats)
    }

    /// The desks above the ground floor kept across a restart, on floors of
    /// `Floor.desks` desks: anyone at a desk past the last (desks 10 to 12 of
    /// a floor, from when every floor had twelve) moves to the first free desk
    /// on the same floor, or else on the next floor up with one, or else on
    /// the nearest floor below with one. One put on their floor is put on the
    /// floor they move to. Their role, nickname and folder's rules go with
    /// them, being kept by session and folder rather than by desk. With no
    /// free desk anywhere, their desk is let go: they sit where there's room,
    /// as anyone new does, or wait in the elevator's list. Nobody disappears.
    /// Nil when nobody needs to move, so seats already moved stay as they are.
    /// The Windows and Linux app's `onFloorDesks` does the same.
    static func onFloorDesks(seats: [String: Seat], floors: [String: Int]) -> (seats: [String: Seat], floors: [String: Int])? {
        let upstairs = 1..<Floor.count
        let movers = seats.filter { upstairs.contains($0.value.floor) && $0.value.desk >= Floor.desks }
            .sorted { ($0.value.floor, $0.value.desk, $0.key) < ($1.value.floor, $1.value.desk, $1.key) }
        guard !movers.isEmpty else { return nil }
        var (seats, floors) = (seats, floors)
        var taken = Set(seats.values.filter { upstairs.contains($0.floor) && (0..<Floor.desks).contains($0.desk) })
        for (id, had) in movers {
            // Its own floor, then those above it, then those below, nearest first
            let order = [had.floor] + Array(had.floor + 1 ..< Floor.count) + (1..<had.floor).reversed()
            let free = order.lazy.compactMap { floor in (0..<Floor.desks).map { Seat(floor: floor, desk: $0) }.first { !taken.contains($0) } }.first
            guard let free else {
                seats[id] = nil
                continue
            }
            taken.insert(free)
            seats[id] = free
            if free.floor != had.floor, floors[id] == had.floor { floors[id] = free.floor }
        }
        return (seats, floors)
    }

    /// The floor a session sits on as things stand: 0 for the ground floor.
    /// Where it was put, if it was; else where its folder's rule says; else
    /// nil, for the ground floor's keys, then the floors above as they fill.
    static func floor(of project: String, put: Int?, rules: [FloorRule]) -> Int? {
        if let put, (0..<Floor.count).contains(put) { return put }
        return FloorRule.floor(of: project, in: rules)
    }
}
