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
        // Desks are always as many as there are sessions now: an older fixed count is read but not kept.
        _ = try c.decodeIfPresent(KeyCount.self, forKey: .keys)
        keys = .auto
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
/// desk, a tent, a throne…), the monitors, the door, what drifts over the room and the chairs' colour, how many of
/// its own staff hang about, the style it's in (Yours, or one that furnishes
/// it for you), your team, the dress code, and where everything goes. office.js
/// draws it, with the same choices. Anything unknown, or of the wrong kind, is
/// as standard: it only changes how the room looks.
struct Room: Codable, Equatable {
    var mic = "mic"
    /// How the mic's sound waves look while it is held: "classic" is the red dotted arcs.
    var wave = "classic"
    /// The emergency button: "nuclear" is the red dome; the others each go off their own way.
    var button = "nuclear"
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
    /// The recorded keyboards while agents work, and talk at a desk a message is taken to, under the rest of the sound.
    var ambience = "on"
    /// Whether the sound carries on with the office behind other windows, minimised or on another Space ("always"),
    /// or only while it's in front ("front").
    var soundWhen = "always"
    var desk = "wood"
    var workspace = "desk"
    var monitor = "standard"
    /// The door onto the corridor: as it comes, haunted, with a wreath, a love-letter door, painted for the Lunar New Year, a locker room's, or a saloon's swinging half-doors.
    var door = "standard"
    /// What drifts, floats or crosses the room now and then: nothing, snow, hearts, the Lunar New Year's dragon dance, or confetti.
    var drift = "none"
    /// What the windows show after dark: nothing, fireworks round midnight, or a burst now and then every night.
    var fireworks = "off"
    var chair = "teal"
    var staff = "some"
    /// How a new session comes in and an ended one goes: "animated", with a coffee or a box, or "plain", just walking.
    var arrivals = "animated"
    /// A calm office: nobody wanders off, the pets rest, nothing drifts by, and it's drawn less often, for less power.
    /// Agents still work, type and come and go.
    var calm = "off"
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
        "wave": ["classic", "orange", "ocean", "mint", "gold", "rainbow", "rings", "dashed", "sine", "eq", "dots", "notes", "hearts", "sparks", "lightning", "speech", "radar", "glitch", "bubbles", "coins", "basketballs"],
        "mic": ["mic", "megaphone", "studio", "podcast", "karaoke", "headset", "walkie", "tincan", "radio", "reporter", "clawdspark", "ribbon", "elvis", "carbon", "condenser", "banana", "tentacle", "cthulhu", "coinmic", "holomic", "ringmic", "spacecomm", "bulbmic", "pumpkinmic"],
        "button": ["nuclear", "mushroom", "glassbox", "keyswitch", "easy", "slots", "disco", "confetti", "gong", "bikehorn", "spark", "buzzer", "goalhorn", "selfdestruct", "hologram", "warp"],
        "floor": Room.floors,
        "paint": Room.paints,
        // The lounge's carpets, or any of the office's floors
        "lounge": ["navy", "teal", "plum", "sand"] + Room.floors,
        "loungePaint": ["auto"] + Room.paints,
        "corner": ["arcade", "vending", "foosball", "fishtank", "books", "dragon", "pinball", "jukebox", "pool", "piano", "drums",
                   "telescope", "jellyfish", "bonsai", "jungle", "coffee", "servers", "printer", "boxing", "treadmill", "hottub",
                   "ballpit", "campfire", "bubbler", "neonbox", "wallbox", "djtable", "turntable", "boombox", "hifi", "synth",
                   "cauldron", "graveyard", "lanterns", "pumpkinrack", "halfcourt", "arcadehoop", "ballrack", "studiodesk", "bench",
                   "coffinbox", "skeletonmodel", "xmastree", "fireplace", "sleigh", "carolbox", "woodstove",
                   "cocoabar", "heartbox", "candledinner", "plumtree", "lionhead", "watchparty", "noodlebar", "periscope", "sonar", "espressobar", "saloonbar", "pianola",
                   "stadiumorgan", "valveradio", "guzheng", "holobooth", "gramophone", "discobooth", "cosmicsynth", "koipond", "singingbowls", "guitarhook", "clawmachine", "trainset", "easel", "orrery", "copier", "teslacoil", "chesstable", "butterflyhouse", "switchboard", "puttinggreen", "retrofridge", "readingchair", "potterywheel", "projector", "birdfeeder", "vivarium", "claudecade", "sparkfloorlamp", "eldritchaltar", "deepaquarium", "miskatonicnook", "coldsafe", "noderack", "bullstatue", "coinfountain", "sleepybear", "leakbucket", "miningrig", "parcstand", "celticsmascot", "bullsmascot", "heatmascot", "spursmascot", "rocketsmascot", "raptorsmascot", "sunsmascot", "nuggetsmascot", "thundermascot", "grizzliesmascot", "hawksmascot", "hornetsmascot", "pacersmascot", "bucksmascot", "wolvesmascot", "jazzmascot", "kingsmascot", "magicmascot", "pistonsmascot", "pelicansmascot", "cavsmascot", "mavsmascot", "sixersmascot", "blazersmascot", "wizardsmascot", "clippersmascot", "netsmascot", "lakersmascot", "knicksmascot", "warriorsmascot", "marvel1worthy", "marvel1webcorner", "marvel2throne", "marvel2herbgarden", "ironmanhulkbuster", "ironmangantry", "batcavebatmobile", "batcavepenny", "dcmixphonebooth", "dcmixtreadmill", "pokemonhealer", "pokemonsnorlax", "lolnexus", "lolbaron", "wowforge", "wowgryphon", "gtapayspray", "gtaphonebooth", "destinycryptarch", "destinysparrowbay", "starwarsholotable", "starwarscantina", "girly1convertible", "girly1wardrobe", "girly2dessert", "girly2plushpile", "narcosmoney", "narcoslab", "apple1geniusbar", "apple1trees", "apple2bigmac", "apple2garage", "jewishsukkah", "jewishstudy", "muslimprayer", "muslimfountain", "christianpew", "christianorgan", "buddhistaltar", "buddhistmeditation", "volcanolavapit", "volcanothrone", "kitchenline", "kitchenpass", "hospitalbed", "hospitalcrash", "noirinterrogation", "noircell", "madlabslab", "madlabtank", "casinoslots", "casinoroulette", "orientdining", "orientloco", "halloweendxcauldron", "halloweendxtree", "catcafetree", "catcafecounter", "spiderversemural", "spiderversegohome", "xmencerebro", "xmensentinelhead", "f1simrig", "f1garage", "ufcheavybag", "ufcoctagon", "nyloftrecords", "nyloftplants", "pariscafeterrace", "pariscafepastry", "muertosofrenda", "muertoscart", "submarinedeepairlock", "submarinedeeptank", "arcticbasewoodstove", "arcticbasesnowmobile", "sakuratree", "sakurastall", "potterplatform", "potterfireplace", "lotrprancing", "lotrdoom", "breakingbadrvcorner", "breakingbadreactor", "strangergate", "strangercastle", "gotironthrone", "gotmaptable", "simpsonstv", "simpsonskwikemart", "bondbaccarat", "bondlaser", "duneworm", "duneconservatory", "onepiecefigurehead", "onepiecegalley", "eldenringgracesite", "eldenringminortree", "sevmde", "sevwellnessroom", "offreception", "offkitchen", "peakybar", "peakyfireplace", "madmbarcart", "madmhifi", "squidredlight", "squidglassbridge", "boysstage", "boysstatue", "falloutvaultcog", "falloutpowerarmor", "rickship", "rickportallab", "mirrorpod", "mirrorserver", "westpianocorner", "westrepairbay", "bttfdelorean", "bttfdinerbooth", "jptrex", "jphatchery", "wickbar", "wickvault", "kingsmanfitting", "kingsmanroundtable", "wonkariver", "wonkagarden", "weslobby", "wesbakery", "mibarrivals", "mibbigscreen", "tronarcade", "troncyclearena", "dbzgravity", "dbzkamehouse", "amongusreactor", "amonguscafeteria", "tokyoyatai", "tokyoshrine", "egyptsarc", "egyptbrazier", "vikinghearth", "vikinghighseat", "aztecshrine", "aztecserpenthead", "obstelescope", "obsplanetarium", "tikibar", "tikifirepit", "dinerjukebox", "dinercounter", "courtbench", "courtjurybox", "arteasel", "artgallery", "aquariumtunnel", "aquariumtouchpool", "circuscannon", "circuscalliope", "museumrex", "museumgem", "northpoletree", "northpolesleigh", "soundstagedolly", "soundstageset", "cultshrine", "cultgathering", "esportsstage", "esportsstream", "cavemanfire", "cavemanmount", "lodgealtar", "lodgethrone", "prepperstation", "prepperstockpile", "paradisevault", "paradisesurveil", "startrektransporter", "startrekwarpcore", "gentlemenlounge", "gentlemenbilliards", "groklaunch", "grokbotrack", "none"],
        "sky": ["live", "day"],
        "season": ["calendar", "off"],
        "sound": ["off", "quiet", "normal"],
        "ambience": ["on", "off"],
        "soundWhen": ["always", "front"],
        "desk": ["wood", "walnut", "white", "black"],
        "workspace": Room.workspaces,
        "monitor": ["standard", "crt", "holo", "brass", "crystal", "gilt", "ice", "ultrawide", "dual", "laptop", "bamboo", "tablet",
                    "portrait", "broadcast", "ghostcrt", "ouija", "tombstone", "candycane", "giftwrap", "knitmon", "heartframe", "locket", "lanternmon", "scoreboard", "catnap", "mossframe", "satdish", "bookstack", "woodcase", "nestmonitor", "claudeterm", "sparksaver", "wirelessset", "scryingscreen", "orderbook", "greencandles", "redcandles", "stickerback", "matchcast", "lakerscast", "celticscast", "warriorscast", "knickscast", "bullscast", "heatcast", "spurscast", "thundercast", "wolvescast", "nuggetscast", "hawkscast", "netscast", "hornetscast", "cavscast", "mavscast", "pistonscast", "rocketscast", "pacerscast", "clipperscast", "grizzliescast", "buckscast", "pelicanscast", "magiccast", "sixerscast", "sunscast", "blazerscast", "kingscast", "raptorscast", "jazzcast", "wizardscast", "marvel1shieldmon", "marvel2kimoyo", "ironmanjarvis", "batcavescreen", "dcmixlanternmon", "pokemondex", "lolhexmon", "wowportal", "gtawanted", "destinytraveler", "starwarsmon", "girly1mon", "girly2mon", "narcosmon", "apple1imac", "apple2classic", "jewishmon", "muslimmon", "christianmon", "buddhistmon", "volcanomon", "kitchenmon", "hospitalmon", "noirmon", "madlabmon", "casinomon", "orientmon", "halloweendxmon", "catcafemon", "spiderversemon", "xmenmon", "f1mon", "ufcmon", "nyloftmon", "pariscafemon", "muertosmon", "submarinedeepmon", "arcticbasemon", "sakuramon", "pottermon", "lotrmon", "breakingbadmon", "strangermon", "gotmon", "simpsonsmon", "bondmon", "dunemon", "onepiecemon", "eldenringmon", "sevmon", "offmon", "peakymon", "madmmon", "squidmon", "boysmon", "falloutmon", "rickmon", "mirrormon", "westmon", "bttftimecircuits", "jpmon", "wickmon", "kingsmanmon", "wonkamon", "wesmon", "mibmon", "tronmon", "dbzmon", "amongusmon", "tokyomon", "egyptmon", "vikingmon", "aztecmon", "obsmon", "tikimon", "dinermon", "courtmon", "artmon", "aquariummon", "circusmon", "museummon", "northpolemon", "soundstagemon", "cultmon", "esportsmon", "cavemanmon", "lodgemon", "preppermon", "paradiseterminal", "startrekconsole", "gentlemenwoodcase", "grokmon", "grokbotmon"],
        "door": ["standard", "haunted", "wreathdoor", "mailboxdoor", "lunardoor", "lockerdoor", "saloondoor", "glassdoor", "dutchdoor", "screendoor", "vaultdoor", "marvel1door", "marvel2sanctumdoor", "ironmandoor", "batcaveclock", "dcmixhalldoor", "pokemondoor", "loldoor", "wowdoor", "gtadoor", "destinydoor", "starwarsdoor", "girly1door", "girly2door", "narcosdoor", "apple1door", "apple2door", "jewishdoor", "muslimdoor", "christiandoor", "buddhistdoor", "volcanodoor", "kitchendoor", "hospitaldoor", "noirdoor", "madlabdoor", "casinodoor", "orientdoor", "halloweendxdoor", "catcafedoor", "spiderversedoor", "xmendoor", "f1door", "ufcdoor", "nyloftdoor", "pariscafedoor", "muertosdoor", "submarinedeepdoor", "arcticbasedoor", "sakuradoor", "potterdoor", "lotrdoor", "breakingbaddoor", "strangerdoor", "gotdoor", "simpsonsdoor", "bonddoor", "dunedoor", "onepiecedoor", "eldenringfog", "sevdoor", "offdoor", "peakydoor", "madmdoor", "squiddoor", "boysdoor", "falloutdoor", "rickdoor", "mirrordoor", "westdoor", "bttfdoor", "jpgatedoor", "wickdoor", "kingsmanfittingdoor", "wonkadoor", "wesdoor", "mibdoor", "trondoor", "dbzdoor", "amongusdoor", "tokyodoor", "egyptdoor", "vikingdoor", "aztecdoor", "obsdoor", "tikidoor", "dinerdoor", "courtdoor", "artdoor", "aquariumdoor", "circusflap", "museumarch", "northpolewreathdoor", "soundstagedoor", "cultdoor", "esportsdoor", "cavemandoor", "lodgedoor", "prepperhatch", "paradiseblastdoor", "startrekturbolift", "gentlemenbookdoor", "grokdoor"],
        "drift": ["none", "snow", "hearts", "lunar", "confetti", "seamist", "greenburst", "marvel1webzip", "marvel2snap", "ironmanflyby", "batcaveswarm", "dcmixspeedster", "pokemonthrow", "lolpings", "wowlevelup", "gtasiren", "destinyengrams", "starwarslightspeed", "girly1glitter", "girly2bubbles", "narcosmoneyrain", "apple1icons", "apple2toasters", "jewishlights", "muslimlanterns", "christianstar", "buddhistleaves", "volcanoembers", "kitchensteam", "hospitalsiren", "noirrain", "madlabbubbles", "casinocardrain", "orientsteam", "halloweendxghost", "catcafeyarnball", "spiderverseswing", "xmenbamf", "f1flyby", "ufcflashes", "nylofttrain", "pariscafenotes", "muertosdrift", "submarinedeepbubbles", "arcticbaseaurora", "sakuraflurry", "potterletters", "lotrfirework", "breakingbadcash", "strangerspores", "gotdragonshadow", "simpsonsdonuts", "bonddots", "dunespicedrift", "onepiecebubbles", "eldenringleaves", "sevballoons", "offplanes", "peakysmoke", "madmsmoke", "squidmoney", "boysstreamers", "falloutdust", "rickspark", "mirrorglitch", "westdust", "bttfsparks", "jptremor", "wickcoinrain", "kingsmanbrolly", "wonkabubbles", "wespostcards", "mibflash", "tronderez", "dbzki", "amonguseject", "tokyorain", "egyptsand", "vikingembers", "aztecleaves", "obsstars", "tikipetals", "dinernotes", "courtpapers", "artsplatter", "aquariumbubbles", "circusrelease", "museumdust", "northpolesleighby", "soundstagebeams", "cultpetals", "esportspyro", "cavemanembers", "lodgeincense", "prepperalert", "paradisedust", "startrekalertdrift", "gentlemensmoke"],
        "fireworks": ["off", "midnight", "nightly"],
        "chair": ["teal", "black", "red", "grey", "green"],
        "staff": ["some", "none", "many"],
        "arrivals": ["animated", "plain"],
        "calm": ["off", "on"],
        "preset": Room.presets,
    ]
    static let floors = ["wood", "tiles", "concrete", "carpet", "plating", "deck", "marble", "flagstone", "snow", "grass", "forest",
                         "rubber", "epoxy", "arcade", "hauntedcourt", "court", "studio", "stationfloor", "zensand", "planks", "wetflags", "polished", "emeraldgold", "worncarpet", "cablefloor", "pitchgrass", "lakerscourt", "celticscourt", "warriorscourt", "knickscourt", "bullscourt", "heatcourt", "spurscourt", "thundercourt", "wolvescourt", "nuggetscourt", "hawkscourt", "netscourt", "hornetscourt", "cavscourt", "mavscourt", "pistonscourt", "rocketscourt", "pacerscourt", "clipperscourt", "grizzliescourt", "buckscourt", "pelicanscourt", "magiccourt", "sixerscourt", "sunscourt", "blazerscourt", "kingscourt", "raptorscourt", "jazzcourt", "wizardscourt", "marvel1tower", "marvel1stars", "marvel2vibranium", "marvel2cosmos", "ironmangarage", "ironmanhexglass", "batcavedeck", "batcaverock", "dcmixwatchtower", "dcmixthemyscira", "pokemontiles", "pokemongrass", "lolhextiles", "lolrift", "wowinn", "wowcobble", "gtavice", "gtaasphalt", "destinytower", "destinyrust", "starwarsdeck", "starwarssand", "girly1marble", "girly1shag", "girly2mint", "girly2cloud", "narcostiles", "narcosleopard", "apple1oak", "apple1stone", "apple2carpet", "apple2dither", "jewishstone", "jewishtiles", "muslimzellige", "muslimcarpet", "christianflags", "christianoak", "buddhisttatami", "buddhistslate", "volcanobasalt", "volcanogrid", "kitchenquarry", "kitchenmat", "hospitallino", "hospitallines", "noirchecker", "noirboards", "madlabdrain", "madlabplate", "casinocarpet", "casinomarble", "orientterrazzo", "orientcarpet", "halloweendxboards", "halloweendxsoil", "catcafeoak", "catcafepatch", "spiderverserooftop", "spiderversehalftone", "xmendanger", "xmenparquet", "f1tarmac", "f1chequer", "ufccanvas", "ufcmats", "nyloftboards", "nyloftrug", "pariscafetiles", "pariscafechevron", "muertostiles", "muertospetals", "submarinedeepgrate", "submarinedeepmat", "arcticbasetread", "arcticbaserug", "sakuraboards", "sakuralawn", "potterflags", "pottercommon", "lotrshire", "lotrmeduseld", "breakingbadlab", "breakingbaddesert", "strangerarcade", "strangerupside", "gotwinterfell", "gotredkeep", "simpsonsplant", "simpsonsliving", "bondmi6", "bondlair", "dunesietch", "dunesand", "onepiecedeck", "onepiecelawn", "eldenringleyndell", "eldenringlimgrave", "sevcarpet", "sevtile", "offcarpet", "offlino", "peakyboards", "peakycobble", "madmcarpet", "madmparquet", "squidfield", "squidteal", "boysmarble", "boyscarpet", "falloutvault", "falloutconcrete", "rickgarage", "rickportal", "mirrorfloor", "mirrordark", "westlab", "westsaloon", "bttfgaragefloor", "bttfdinerfloor", "jpvisitorfloor", "jpjunglefloor", "wicklobbyfloor", "wickvaultfloor", "kingsmanshopfloor", "kingsmanhqfloor", "wonkariverfloor", "wonkacandyfloor", "weslobbyfloor", "wescorridorfloor", "mibhqfloor", "mibtunnelfloor", "trongridfloor", "tronarenafloor", "dbzdojofloor", "dbzlabfloor", "amongusshipfloor", "amonguscafefloor", "tokyofloor", "tokyotatami", "egyptfloor", "egyptmosaic", "vikingfloor", "vikingstone", "aztecfloor", "aztecjungle", "obsfloor", "obscarpet", "tikifloor", "tikisand", "dinerfloor", "dinerterrazzo", "courtfloor", "courtcarpet", "artfloor", "artrug", "aquariumfloor", "aquariumglass", "circussawdust", "circusredcarpet", "museummarble", "museumparquet", "northpolesnow", "northpoleplank", "soundstagefloor", "soundstagelounge", "cultclay", "cultmat", "esportsstagefloor", "esportscarpet", "cavemanrock", "cavemanhide", "lodgechecker", "lodgecarpet", "prepperconcrete", "prepperrubber", "paradiseovalrug", "paradisebunkerfloor", "startrekdeck", "startrekpad", "gentlemenherringbone", "gentlementartan", "grokfloor", "grokrug", "grokbotfloor"]
    static let paints = ["slate", "sage", "clay", "cream", "hull", "timber", "damask", "stone", "ice", "logs", "brick", "bamboo",
                         "block", "foam", "primetime", "bleachers", "stationwall", "shoji", "plankwall", "deepsea", "fintech", "greenglow", "frostblue", "ledstrip", "parcnavy", "lakerswall", "celticswall", "warriorswall", "knickswall", "bullswall", "heatwall", "spurswall", "thunderwall", "wolveswall", "nuggetswall", "hawkswall", "netswall", "hornetswall", "cavswall", "mavswall", "pistonswall", "rocketswall", "pacerswall", "clipperswall", "grizzlieswall", "buckswall", "pelicanswall", "magicwall", "sixerswall", "sunswall", "blazerswall", "kingswall", "raptorswall", "jazzwall", "wizardswall", "marvel1red", "marvel1webwall", "marvel2wakanda", "marvel2sanctum", "ironmanred", "ironmanglass", "batcavestone", "batcavemanor", "dcmixsuperblue", "dcmixlanternwall", "pokemoncenter", "pokemonroute", "lolpiltover", "loljungle", "wowstormwind", "wowhorde", "gtavicecity", "gtabrick", "destinylastcity", "destinyvoid", "starwarsimperial", "starwarsrebel", "girly1hotpink", "girly1blush", "girly2lilac", "girly2mintwall", "narcoshacienda", "narcosjungle", "apple1white", "apple1space", "apple2beige", "apple2rainbow", "jewishblue", "jewishsand", "muslimteal", "muslimplaster", "christianwhite", "christianwine", "buddhistsaffron", "buddhistmaroon", "volcanorock", "volcanosteel", "kitchentile", "kitchenbrick", "hospitalmint", "hospitalwhite", "noirplaster", "noirgreen", "madlabstone", "madlabtile", "casinored", "casinoneon", "orientmarquetry", "orientcream", "halloweendxdamask", "halloweendxcrypt", "catcafepink", "catcafemint", "spiderversegraffiti", "spiderversegwen", "xmenmansion", "xmenhangar", "f1pitwall", "f1carbon", "ufcarena", "ufcgym", "nyloftbrick", "nyloftplaster", "pariscafeboiserie", "pariscafebordeaux", "muertospink", "muertosteal", "submarinedeepwhite", "submarinedeepnavy", "arcticbasered", "arcticbasepine", "sakurablush", "sakurasky", "pottergryffindor", "potterstone", "lotrbagend", "lotrrivendell", "breakingbadsuperlab", "breakingbadstucco", "strangerwallpaper", "strangerupsidewall", "gotstone", "gotcrimson", "simpsonspink", "simpsonssky", "bondwalnut", "bondgunwall", "duneatreides", "dunerock", "onepiecesunny", "onepiecesea", "eldenringstormveil", "eldenringerdtree", "sevwall", "sevhall", "offwall", "offbreak", "peakywall", "peakybrick", "madmwall", "madmteak", "squidstairs", "squidpink", "boyswall", "boysred", "falloutwall", "falloutgrime", "rickwall", "rickalien", "mirrorwall", "mirrordarkwall", "westlabwall", "westsaloonwall", "bttfgaragewall", "bttfdinerwall", "jpconcretewall", "jpjunglewall", "wicklobbywall", "wickvaultwall", "kingsmanshopwall", "kingsmanhqwall", "wonkafactorywall", "wonkagardenwall", "weslobbywall", "wescorridorwall", "mibhqwall", "mibtunnelwall", "trongridwall", "tronorangewall", "dbzdojowall", "dbzlabwall", "amongusshipwall", "amonguscafewall", "tokyoneonwall", "tokyobrick", "egyptwall", "egyptgold", "vikingwall", "vikingrune", "aztecwall", "aztecvine", "obswall", "obspanel", "tikiwall", "tikisunset", "dinerwall", "dinerstripe", "courtwall", "courtmarble", "artwall", "artbrick", "aquariumwall", "aquariumcoral", "circusbigtop", "circusmarquee", "museumstone", "museumgallery", "northpolelog", "northpolewrap", "soundstageacoustic", "soundstagegreenroom", "cultadobe", "cultveil", "esportsled", "esportspanel", "cavemanwall", "cavemanpaintings", "lodgepanel", "lodgedrape", "prepperwall", "preppersandbag", "paradiseoval", "paradiseconcrete", "startrekbulkhead", "startrekjefferies", "gentlemenpanel", "gentlementweed", "grokwall", "grokpanel", "grokbotwall"]
    /// The office's styles: Yours, as you furnished it, then each that
    /// furnishes it for you. office.js keeps what each one is.
    static let presets = ["yours", "classic", "campsite", "royal", "space", "pirates", "arctic", "lair", "gamers", "garage", "gym", "jungle",
                          "neoncity", "submarine", "lofi", "saloon", "zen", "treehouse", "lovecraft", "claudeteam", "cryptooffice", "bullrun", "bearmarket", "cryptoden", "psg", "lakers", "celtics", "warriors", "knicks", "bulls", "heat", "spurs", "thunder", "wolves", "nuggets", "hawks", "nets", "hornets", "cavs", "mavs", "pistons", "rockets", "pacers", "clippers", "grizzlies", "bucks", "pelicans", "magic", "sixers", "suns", "blazers", "kings", "raptors", "jazz", "wizards", "marvel1", "marvel2", "ironman", "batcave", "dcmix", "pokemon", "lol", "wow", "gta", "destiny", "starwars", "girly1", "girly2", "narcos", "apple1", "apple2", "jewish", "muslim", "christian", "buddhist", "ironmanhall", "volcano", "kitchen", "hospital", "noir", "madlab", "casino", "orient", "halloweendx", "catcafe", "spiderverse", "xmen", "f1", "ufc", "nyloft", "pariscafe", "muertos", "submarinedeep", "arcticbase", "sakura", "potter", "lotr", "breakingbad", "stranger", "got", "simpsons", "bond", "dune", "onepiece", "eldenring", "sev", "off", "peaky", "madm", "squid", "boys", "fallout", "rick", "mirror", "west", "bttf", "jp", "wick", "kingsman", "wonka", "wes", "mib", "tron", "dbz", "amongus", "tokyo", "egypt", "viking", "aztec", "obs", "tiki", "diner", "court", "art", "aquarium", "circus", "museum", "northpole", "soundstage", "cult", "esports", "caveman", "lodge", "prepper", "paradise", "startrek", "gentlemen", "grok", "grokbots"]
    /// What an agent can work at: the office's choice for everyone, and a
    /// team member's own, which can also be "auto".
    static let workspaces = ["desk", "standing", "gaming", "tent", "box", "beanbag", "igloo", "van", "cockpit", "ship", "throne",
                             "greenhouse", "console", "hover", "chest", "gilded", "crates", "altar", "log", "bench", "bamboo", "rack",
                             "drafting", "glass", "scorer", "anchor", "coffin", "waxdesk", "clawdesk", "gingerdesk", "workbench", "plaiddesk", "pinkdesk", "lacedesk", "lacquerdesk", "courtdesk", "cyberdesk", "subconsole", "cafetable", "barreldesk", "discodesk", "lowtable", "cratedesk", "scriptorium", "charttable", "tradingdesk", "bulldesk", "heaterdesk", "screenwall", "pitchdesk", "lakersdesk", "celticsdesk", "warriorsdesk", "knicksdesk", "bullsdesk", "heatdesk", "spursdesk", "thunderdesk", "wolvesdesk", "nuggetsdesk", "hawksdesk", "netsdesk", "hornetsdesk", "cavsdesk", "mavsdesk", "pistonsdesk", "rocketsdesk", "pacersdesk", "clippersdesk", "grizzliesdesk", "bucksdesk", "pelicansdesk", "magicdesk", "sixersdesk", "sunsdesk", "blazersdesk", "kingsdesk", "raptorsdesk", "jazzdesk", "wizardsdesk", "marvel1deck", "marvel2milano", "ironmanbench", "batcaveconsole", "dcmixplanetdesk", "pokemonballdesk", "lolhexdesk", "wowquestdesk", "gtalowrider", "destinyvanguard", "starwarsconsole", "girly1vanity", "girly2desk", "narcosdesk", "apple1table", "apple2desk", "jewishdesk", "muslimdesk", "christiandesk", "buddhistdesk", "volcanodesk", "kitchendesk", "hospitaldesk", "noirdesk", "madlabdesk", "casinodesk", "orientdesk", "halloweendxdesk", "catcafedesk", "spiderversedesk", "xmendesk", "f1desk", "ufcdesk", "nyloftdesk", "pariscafedesk", "muertosdesk", "submarinedeepdesk", "arcticbasedesk", "sakuradesk", "potterdesk", "lotrdesk", "breakingbaddesk", "strangerdesk", "gotdesk", "simpsonsconsole", "bonddesk", "dunedesk", "onepiecedesk", "eldenringdesk", "sevdesk", "offdesk", "peakydesk", "madmdesk", "squiddesk", "boysdesk", "falloutdesk", "rickdesk", "mirrordesk", "westdesk", "bttfbench", "jpcontroldesk", "wickconcierge", "kingsmancuttingtable", "wonkadesk", "wesconcierge", "mibdesk", "trondesk", "dbzdesk", "amongusdesk", "tokyodesk", "egyptdesk", "vikingdesk", "aztecdesk", "obsdesk", "tikidesk", "dinerdesk", "courtcounsel", "artdesk", "aquariumdesk", "circusdesk", "museumdesk", "northpolebench", "soundstagedesk", "cultdesk", "esportsrig", "cavemanslab", "lodgedesk", "prepperdesk", "paradiseresolute", "startrekbridge", "gentlemenpartners", "grokdesk", "grokbotpod"]
    static let wallChoices = ["window", "whiteboard", "pinboard", "poster", "shelf", "plant", "bare",
                              "viewport", "porthole", "panel", "pipes", "seaPorthole", "map", "flag", "cutlasses", "tapestry",
                              "portrait", "stained", "shield", "frost", "skis", "weather", "torch", "arch", "neon", "tv", "hex",
                              "pegboard", "roller", "bike", "mirror", "timer", "climbing", "vines", "tiki", "blind", "cabin", "trail",
                              "paddles", "pumpkins", "lanternhang", "cobweb", "ghosts", "broom", "leafgarland", "hauntedhoop", "spookyboard",
                              "candycorn", "hoop", "scorebug", "lowerthird", "onair", "schedule", "pennants", "retired",
                              "playboard", "garland", "scarfhooks", "heartgarland", "bemine", "lunarlanterns", "pennantline", "scarfwall", "rainwindow", "neonsign", "fishporthole", "chalkmenu", "pendantlamps", "wanted", "countdown", "streamers", "balloons", "earthwindow", "paperscreen", "leafwindow", "ropeladder", "cuckoo", "photostring", "dartboard", "splitflap", "periodictable", "growshelf", "mugrack", "wallfan", "kite", "windchime", "claudeneon", "modelposter", "hellocard", "contextclock", "tentacleporthole", "starchartodd", "followingportrait", "sunkencity", "eldertapestry", "tickerboard", "candlechart", "marketclocks", "moonposter", "upchart", "moonwindow", "redchart", "stormpane", "hanginthere", "blackout", "denposters", "floodlights", "framedjersey", "lakersbanner", "celticsbanner", "warriorsbanner", "knicksbanner", "bullsbanner", "heatbanner", "spursbanner", "thunderbanner", "wolvesbanner", "nuggetsbanner", "hawksbanner", "netsbanner", "hornetsbanner", "cavsbanner", "mavsbanner", "pistonsbanner", "rocketsbanner", "pacersbanner", "clippersbanner", "grizzliesbanner", "bucksbanner", "pelicansbanner", "magicbanner", "sixersbanner", "sunsbanner", "blazersbanner", "kingsbanner", "raptorsbanner", "jazzbanner", "wizardsbanner", "marvel1shieldwall", "marvel1hammerwall", "marvel1spidewindow", "marvel1assemble", "marvel1hulkhole", "marvel1bugle", "marvel2gauntlet", "marvel2sanctumwindow", "marvel2portal", "marvel2awesomemix", "marvel2tapestry", "marvel2lokihelm", "ironmanarmory", "ironmanblueprint", "ironmanmark1", "ironmanexpo", "ironmanheart", "ironmanpegboard", "batcavesignal", "batcavesuits", "batcavestalactites", "batcaveboard", "batcavecard", "batcavebatarangs", "dcmixplanetglobe", "dcmixshield", "dcmixlasso", "dcmixflashwindow", "dcmixtrident", "dcmixharley", "pokemonbadges", "pokemonmap", "pokemonwindow", "pokemonposter", "pokemonballshelf", "pokemonsign", "lolminimap", "lolbanner", "lolcrest", "lolwindow", "lolabilities", "lolpenta", "wowlion", "wowhordebanner", "wowquestboard", "wowwindow", "wowshield", "wowmap", "gtasunset", "gtamap", "gtaneon", "gtaposter", "gtagrove", "gtaplates", "destinywindow", "destinyhunter", "destinytitan", "destinywarlock", "destinydirector", "destinyxur", "starwarshyper", "starwarssuns", "starwarssabers", "starwarscarbonite", "starwarsbanner", "starwarsplans", "girly1mirror", "girly1neon", "girly1heels", "girly1carposter", "girly1heartclock", "girly1curtains", "girly2rainbow", "girly2kittyframe", "girly2polaroids", "girly2moon", "girly2plushshelf", "girly2cloudclock", "narcoswanted", "narcosmap", "narcosportrait", "narcospalms", "narcossafe", "narcosjersey", "apple1logo", "apple1window", "apple1poster", "apple1phones", "apple1watches", "apple1hello", "apple2logo", "apple2think", "apple21984", "apple2floppy", "apple2macpaint", "apple2bomb", "jewisharch", "jewishbooks", "jewishpapercut", "jewishhanukkiah", "jewishshabbat", "jewishshofar", "muslimmashrabiya", "muslimlantern", "muslimtilepanel", "muslimarabesque", "muslimarch", "muslimquranshelf", "christianstainedglass", "christiancross", "christianrosewindow", "christianbibleshelf", "christiansconce", "christianwreath", "buddhistflags", "buddhistaltarshelf", "buddhistlotusscroll", "buddhistmoonwindow", "buddhistwheel", "buddhistlanterns", "volcanocaldera", "volcanomap", "volcanobeacon", "volcanopipes", "volcanoporthole", "volcanocountdown", "kitchenrail", "kitchenpans", "kitchenknives", "kitchenheatlamps", "kitchenhood", "kitchenspices", "hospitalecg", "hospitalxray", "hospitaleyechart", "hospitalclock", "hospitalexit", "hospitalgloves", "noirblinds", "noirevidence", "noirheight", "noircitymap", "noirhatrack", "noirfrosted", "madlablever", "madlabladder", "madlabjars", "madlabchalk", "madlabgauges", "madlabwindow", "casinomarquee", "casinowheel", "casinoneonsign", "casinoflush", "casinojackpot", "casinochandelier", "orientclock", "orientwindow", "orientposter", "orientluggage", "orientsconce", "orientroute", "halloweendxmoon", "halloweendxportrait", "halloweendxclock", "halloweendxgarland", "halloweendxshelf", "halloweendxmirror", "catcafeshelves", "catcafeportraits", "catcafewindow", "catcafebunting", "catcafeyarn", "catcafeplanter", "spiderversewindow", "spiderversecomic", "spiderverseportal", "spiderversemasks", "spiderversespot", "spiderversepennant", "xmenlogo", "xmenblackbird", "xmenroster", "xmenschool", "xmenalert", "xmenhelmet", "f1lights", "f1tyres", "f1helmetcase", "f1timing", "f1circuit", "f1trophies", "ufcfence", "ufcposter", "ufcbelt", "ufcclock", "ufcgloves", "ufcspeedbag", "nyloftwindow", "nyloftsubway", "nyloftshelf", "nyloftneon", "nyloftclock", "nyloftbike", "pariscafewindow", "pariscafemenu", "pariscafemirror", "pariscafeposter", "pariscafeclock", "pariscafebottles", "muertospapel", "muertosretablo", "muertoscatrina", "muertostinsun", "muertosguitar", "muertoswindow", "submarinedeepviewport", "submarinedeepdepth", "submarinedeepmap", "submarinedeepvalves", "submarinedeeplocker", "submarinedeepsamples", "arcticbasewindow", "arcticbasemap", "arcticbaseweather", "arcticbaseradio", "arcticbaseskis", "arcticbasechart", "sakurabranch", "sakurawindow", "sakurafan", "sakurastreamers", "sakuraprint", "sakuracalendar", "potterportrait", "potterhousepoints", "potterwindow", "potterbroom", "pottermap", "potterbanners", "lotrwindow", "lotrmap", "lotreye", "lotrsting", "lotrgondor", "lotrrohan", "breakingbadlogo", "breakingbadpollos", "breakingbadheisenberg", "breakingbadsaul", "breakingbadwindow", "breakingbadmasks", "strangerlights", "strangerhellfire", "strangerwindow", "strangerscoops", "strangerclock", "strangerhawkins", "gotstark", "gotlannister", "gottargaryen", "gotwindow", "gotswords", "gotweirwood", "simpsonsboat", "simpsonslardlad", "simpsonskrusty", "simpsonswindow", "simpsonsaccident", "simpsonsduff", "bondgunbarrel", "bond007", "bondworldmap", "bondwindow", "bondgadgets", "bondparachute", "dunewindow", "duneatreidesbanner", "duneharkonnen", "dunebullhead", "dunecrysknife", "dunespicemap", "onepiecejollyroger", "onepiecewanted", "onepieceporthole", "onepiecehelm", "onepiecemap", "onepieceswords", "eldenringerdtreewindow", "eldenringsigil", "eldenringmap", "eldenringmoonveil", "eldenringranni", "eldenringdied", "sevkierportrait", "sevprinciples", "sevclock", "sevbreakroom", "sevbanner", "sevlight", "offwindow", "offmotivation", "offwhiteboard", "offcorkboard", "offclock", "offdundieplaque", "peakyoddsboard", "peakywindow", "peakyportrait", "peakywhiskyshelf", "peakycap", "peakyclock", "madmwindow", "madmart", "madmclock", "madmboard", "madmshelf", "madmposter", "squidshapes", "squidpiggybank", "squidladders", "squidnumbers", "squidguardframe", "squidclock", "boyswindow", "boysseven", "boysstat", "boyslogo", "boysaward", "boysclock", "falloutvaultdoor", "falloutvaultboy", "falloutposter", "falloutterminal", "falloutsign", "falloutclock", "rickportalwall", "rickpegboard", "rickchalkboard", "rickjars", "rickflag", "rickclock", "mirrorscreen", "mirrorrating", "mirrorwindow", "mirrorblocked", "mirrorart", "mirrorclock", "westhostvat", "westvitruvian", "westwindow", "westmaze", "westwanted", "westantlers", "bttfclocktower", "bttfneondiner", "bttfroadsign", "bttfsaveclock", "bttfmenuboard", "bttfwallclock", "jpgate", "jpbanner", "jpwarnfence", "jpparkmap", "jpfossilwall", "jppaddocksign", "wickneon", "wickportrait", "wickclock", "wickmirror", "wickboard", "wicksconce", "kingsmantriplemirror", "kingsmansuitrack", "kingsmanfabrics", "kingsmanportrait", "kingsmanclock", "kingsmanshears", "wonkagates", "wonkapipes", "wonkawallpaper", "wonkamarquee", "wonkacandyshelf", "wonkaclock", "wesportrait", "wesclock", "wessunburst", "wespigeonholes", "weslandscape", "wesbanner", "mibworldmap", "mibscreens", "mibzoneclocks", "mibposter", "mibtubes", "mibwanted", "trongrid", "tronrecognizer", "trontower", "tronmap", "tronportal", "tronleaderboard", "dbzturtlekanji", "dbzbanner", "dbzcapsulesign", "dbzscroll", "dbzpunchboard", "dbzclock", "amongusvent", "amongusmap", "amonguswiring", "amongusviewport", "amongustasklist", "amonguswarning", "tokyosign", "tokyolanterns", "tokyonoren", "tokyobillboard", "tokyovending", "tokyocables", "egypthiero", "egyptanubis", "egyptpharaoh", "egypttorch", "egyptankh", "egyptfalcon", "vikingshield", "vikingantlers", "vikingtapestry", "vikinghornrack", "vikingrunestone", "vikingtorch", "aztecsunstone", "aztecidol", "aztecserpent", "aztecmask", "aztecvines", "aztecglyphs", "obschart", "obsconstellation", "obsmoonphase", "obszodiac", "obsplanetposter", "obssextant", "tikimask", "tikisurfboard", "tikitorch", "tikipuffer", "tikisign", "tikihula", "dinerneon", "dinerclock", "dinermenu", "dinerpie", "dinershake", "dinerplates", "courtscales", "courtflag", "courtseal", "courtclock", "courtbooks", "courtportrait", "artcanvas", "artpalette", "artsketch", "artskylight", "artshelf", "artposter", "aquariumreef", "aquariumjelly", "aquariumshark", "aquariumseahorse", "aquariumstar", "aquariumtube", "circusposter", "circusbunting", "circussign", "circustrapeze", "circuswheel", "circusmirror", "museumtrex", "museumdiorama", "museumbutterflies", "museumgems", "museumportrait", "museummap", "northpolewreath", "northpolestockings", "northpolewindow", "northpolelist", "northpolelights", "northpolegears", "soundstagerig", "soundstageposter", "soundstagescreen", "soundstagestar", "soundstagestoryboard", "soundstagereel", "culteye", "cultsun", "cultportrait", "culttenets", "cultwindow", "cultcandles", "esportsscreen", "esportsbracket", "esportsleaderboard", "esportsbanner", "esportsneon", "esportsshelf", "cavemanhandprints", "cavemanhunt", "cavemanbones", "cavemanwheel", "cavemanwindow", "cavemantorch", "lodgeeye", "lodgepillars", "lodgeportrait", "lodgecharter", "lodgesymbols", "lodgesconce", "prepperboard", "preppershelf", "prepperhazard", "preppertools", "prepperclock", "preppervent", "paradisesealplaque", "paradisefakewindow", "paradisemonitors", "paradiseblasthatch", "paradisedoomsday", "paradiseportrait", "startrekviewscreen", "startreklcars", "startrekplaque", "startrekalert", "startrekstarmap", "startrekport", "gentlemenstag", "gentlemenportrait", "gentlemenguncabinet", "gentlemenmap", "gentlemengrowroom", "gentlemenhunt", "grokneon", "grokgalaxy", "grok42", "grokboard", "grokwindow", "grokfun", "grokbotmissions", "grokbotroutine", "grokbotmailbox", "grokbotstatus"]
    static let standardWall = ["window", "whiteboard", "window", "pinboard", "window"]
    /// The pets an office can have, and the most it has at once.
    static let pets = ["cat", "dog", "lizard", "monkey", "llama", "penguin", "parrot", "owl", "capybara", "redpanda", "corgi", "hamster",
                       "robodog", "bat", "snail", "fox", "axolotl", "blackcat", "goat", "raven", "ghost", "spider", "reindeer", "snowman",
                       "snowhare", "cupid", "lovebirds", "drone", "tumbleweed", "hedgehog", "tortoise", "treefrog", "marsrover", "ferret", "duckling", "tinrobot", "guineapig", "robovac", "hermitcrab", "hen", "sparkling", "shoggoth", "minicthulhu", "rgbcat", "marvel1jeff", "marvel1throg", "marvel2groot", "marvel2goose", "ironmandummy", "ironmanlegion", "batcaveace", "batcavecow", "dcmixkrypto", "dcmixstreaky", "pokemonpikachu", "pokemoncharmander", "lolporo", "lolminion", "wowmurloc", "wowwhelp", "gtachop", "gtapigeon", "destinyghost", "destinyservitor", "starwarsr2", "starwarsgrogu", "girly1poodle", "girly1flamingo", "girly2kitty", "girly2bunny", "narcoshippo", "narcosparrot", "apple1siri", "apple1ipod", "apple2mac", "apple2dogcow", "jewishdove", "jewishhoopoe", "muslimcat", "muslimkitten", "christianlamb", "christianrobin", "buddhistdeer", "buddhistkoi", "volcanosalamander", "volcanocat", "volcanogolem", "kitchencat", "kitchenlobster", "kitchenmouse", "hospitaldog", "hospitalbot", "hospitalbunny", "noircat", "noirpigeon", "noirhound", "madlabrat", "madlabslime", "madlabbrainbot", "casinopug", "casinotiger", "casinodove", "orientscottie", "orientsiamese", "orientpeke", "halloweendxwisp", "halloweendxpumpkin", "halloweendxbatcat", "halloweendxghostdog", "catcafeginger", "catcafecalico", "catcafeblue", "catcafetabby", "catcafepersian", "catcafetortie", "catcafesphynx", "catcafekitten", "catcafemaine", "catcafebengal", "catcafepoodle", "spiderverseham", "spiderversespider", "spiderversespdr", "xmenlockheed", "xmensentinel", "f1roscoe", "f1drone", "ufcgymcat", "ufcpug", "nyloftcat", "nyloftpigeon", "nyloftdachshund", "pariscafepoodle", "pariscafesparrow", "pariscafecat", "muertosxolo", "muertosalebrije", "muertosmonarch", "submarinedeepoctopus", "submarinedeepjelly", "submarinedeepangler", "arcticbasehusky", "arcticbasefox", "arcticbasepuffin", "sakurashiba", "sakuramejiro", "sakurabunny", "potterhedwig", "pottercrookshanks", "potterfawkes", "lotrgollum", "lotreagle", "lotrbill", "breakingbadfly", "breakingbadtortoise", "breakingbadroadrunner", "strangerdemodog", "strangermews", "strangerdemobat", "gotdragon", "gotdirewolf", "gotraven", "simpsonsdog", "simpsonssnowball", "simpsonsplopper", "bondpersian", "bondrc", "bondgyro", "dunemouse", "dunesandworm", "duneornithopter", "onepiecechopper", "onepiecesnail", "onepiecegull", "eldenringtorrent", "eldenringjellyfish", "eldenringpastor", "sevgoat", "sevkier", "offcat", "offbat", "peakyhorse", "peakyterrier", "madmcat", "madmdachshund", "squiddoll", "squidpiggy", "boysbulldog", "boysoctopus", "falloutdog", "falloutmrhandy", "rickmeeseeks", "ricksnowball", "mirrordog", "mirrorbee", "westhorse", "westfly", "bttfeinstein", "bttfdinermouse", "jpraptor", "jpcompy", "wickdog", "wickcat", "kingsmanpug", "kingsmanpigeon", "wonkasquirrel", "wonkagummy", "wescat", "wesfox", "mibpug", "mibroach", "tronbit", "tronpup", "dbzdragon", "dbzturtle", "amonguspet", "amongusufo", "tokyocat", "tokyotanuki", "egyptcat", "egyptscarab", "vikingraven", "vikinggoat", "aztecjaguar", "aztecmacaw", "obsowl", "obscat", "tikiparrot", "tikicrab", "dinercat", "dinerpup", "courtowl", "courtcat", "artcat", "artbird", "aquariumpenguin", "aquariumotter", "circusmonkey", "circusseal", "circuspony", "museumdodo", "museumtortoise", "museummoth", "northpolereindeer", "northpolehusky", "northpolerobin", "soundstagedog", "soundstagecat", "soundstagepigeon", "cultdove", "cultgoose", "cultcat", "esportsmascot", "esportsdrone", "esportscat", "cavemandino", "cavemanmammoth", "cavemanptero", "lodgeraven", "lodgeowl", "lodgecat", "preppercanary", "prepperrat", "prepperdog", "paradisedog", "paradisedrone", "startrektribble", "startrekcat", "gentlemenhound", "gentlemenpheasant", "grokdrone"]
    static let petsMost = 3

    init() {}

    init(from decoder: Decoder) throws {
        let c = try? decoder.container(keyedBy: CodingKeys.self)
        func choice(_ key: CodingKeys) -> String? {
            let given = (try? c?.decodeIfPresent(String.self, forKey: key)) ?? nil
            return given.flatMap { Room.choices[key.stringValue]?.contains($0) == true ? $0 : nil }
        }
        mic = choice(.mic) ?? mic
        wave = choice(.wave) ?? wave
        button = choice(.button) ?? button
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
        ambience = choice(.ambience) ?? ambience
        soundWhen = choice(.soundWhen) ?? soundWhen
        desk = choice(.desk) ?? desk
        workspace = choice(.workspace) ?? workspace
        monitor = choice(.monitor) ?? monitor
        door = choice(.door) ?? door
        drift = choice(.drift) ?? drift
        fireworks = choice(.fireworks) ?? fireworks
        chair = choice(.chair) ?? chair
        staff = choice(.staff) ?? staff
        arrivals = choice(.arrivals) ?? arrivals
        calm = choice(.calm) ?? calm
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
                   "deepseek", "kimi", "glm", "phi", "witch", "skeleton", "mummy", "refskeleton", "mummyjersey",
                   "jersey", "warmup", "referee", "coachpolo", "blazer", "werewolf", "santa", "xmasjumper", "beanie", "cableknit", "redsuit", "heartsweater", "tangsuit", "playoffjersey", "superfan", "techwear", "submariner", "barista", "cowboy", "partyhat", "sequin",
                   // The roles'
                   "artdirector", "lawyer", "therapist", "plumber", "gmexec", "gmcoach", "gmtycoon",
                   "detective", "hardhat", "librarian", "janitor", "firefighter", "pilot", "postman", "accountant", "marketer",
                   "securityguard", "chef", "spacesuit", "kimono", "cultistrobe", "investigator", "deepone", "professor", "divingsuit", "fleecevest", "bullparty", "blanketcape", "denhoodie", "homejersey", "lakersjersey", "celticsjersey", "warriorsjersey", "knicksjersey", "bullsjersey", "heatjersey", "spursjersey", "thunderjersey", "wolvesjersey", "nuggetsjersey", "hawksjersey", "netsjersey", "hornetsjersey", "cavsjersey", "mavsjersey", "pistonsjersey", "rocketsjersey", "pacersjersey", "clippersjersey", "grizzliesjersey", "bucksjersey", "pelicansjersey", "magicjersey", "sixersjersey", "sunsjersey", "blazersjersey", "kingsjersey", "raptorsjersey", "jazzjersey", "wizardsjersey", "marvel1cap", "marvel1spidey", "marvel1thor", "marvel1widow", "marvel2starlord", "marvel2strange", "marvel2panther", "marvel2loki", "ironmanmk3", "ironmanmk2", "ironmanwarmachine", "batcavebatsuit", "batcaverobin", "batcavealfred", "batcavecatwoman", "dcmixsuperman", "dcmixwonder", "dcmixflash", "dcmixgreenlantern", "pokemontrainer", "pokemonrocket", "pokemonnurse", "pokemonoak", "loljinx", "lolahri", "lolgaren", "lollux", "wowpaladin", "wowgrunt", "wowmage", "wowelf", "gtatommy", "gtacj", "gtacop", "gtaheist", "destinyhunterwear", "destinytitanwear", "destinywarlockwear", "destinyexo", "starwarsjedi", "starwarsvader", "starwarsstormtrooper", "starwarsprincess", "girly1gingham", "girly1cowgirl", "girly1tracksuit", "girly1gown", "girly2kittyhood", "girly2bunnyhood", "girly2sailor", "girly2overalls", "narcosboss", "narcosdea", "narcosmiami", "narcoshawaiian", "apple1turtleneck", "apple1genius", "apple1headset", "apple1hoodie", "apple2bowtie", "apple2woz", "apple2rainbowtee", "apple2runner", "jewishkippah", "jewishtichel", "jewishknit", "jewishdress", "muslimhijab", "muslimabaya", "muslimthobe", "muslimkurta", "christianpendant", "christianchoir", "christiansuit", "christianlace", "buddhistrobe", "buddhistmaroonrobe", "buddhistlinen", "buddhistmala", "ironmantony2", "ironmantony2b", "ironmantony2c", "ironmanmk1", "ironmanmk4", "ironmanmk5", "ironmanmk6", "ironmanmk7", "ironmanmk15", "ironmanmk16", "ironmanmk17", "ironmanmk33", "ironmanmk38", "ironmanmk42", "ironmanmk43", "ironmanmk45", "ironmanmk46", "ironmanmk47", "ironmanmk50", "ironmanmk85", "ironmanpatriot", "ironmanironspider", "ironmanrescue", "volcanojumpsuit", "volcanovillain", "volcanoheatsuit", "volcanoguard", "kitchenchef", "kitchensous", "kitchenpartie", "kitchencommis", "hospitalscrubs", "hospitaldoctor", "hospitalnurse", "hospitalsurgeon", "noirtrench", "noiruniform", "noirsuspenders", "noirreporter", "madlabcoat", "madlabapron", "madlabassistant", "madlabhazmat", "casinodealer", "casinotux", "casinohighroller", "casinoshowman", "orientconductor", "orientattendant", "orienttraveller", "orientpearls", "halloweendxvampire", "halloweendxplague", "halloweendxscarecrow", "halloweendxspider", "catcafebarista", "catcafeears", "catcafejumper", "catcafeoveralls", "spiderversemiles", "spiderversegwensuit", "spiderversepeterb", "spiderversepunk", "xmenwolverine", "xmencyclops", "xmengambit", "xmenstormsuit", "f1racesuit", "f1silversuit", "f1pitcrew", "f1teampolo", "ufcrashguard", "ufcreferee", "ufccornerhoodie", "ufcwalkout", "nyloftpeacoat", "nyloftflannel", "nyloftturtleneck", "nyloftleather", "pariscafemariniere", "pariscafegarcon", "pariscafetrench", "pariscafepearls", "muertoscatrinadress", "muertoscharro", "muertosblouse", "muertosbones", "submarinedeepcoverall", "submarinedeepwetsuit", "submarinedeeplabcoat", "submarinedeepsweater", "arcticbaseparka", "arcticbasefleece", "arcticbasecoldsuit", "arcticbaseknit", "sakurayukata", "sakuracardigan", "sakurasailor", "sakuraflorist", "pottergryffindor", "potterslytherin", "potterquidditch", "potterheadmaster", "lotrgandalf", "lotrhobbit", "lotrelf", "lotrranger", "breakingbadhazmat", "breakingbadheisenberg", "breakingbadjesse", "breakingbadsaul", "strangereleven", "strangerhellfiretee", "strangerscoopsuniform", "strangerhopper", "gotnightswatch", "gotlannisterred", "gotdaenerys", "gotwhitewalker", "simpsonshomer", "simpsonsmarge", "simpsonsbart", "simpsonslisa", "bondtux", "bondwhitejacket", "bondmoneypenny", "bondoddjob", "dunestillsuit", "dunefremen", "dunebenegesserit", "duneharkonnenblack", "onepieceluffy", "onepiecezoro", "onepiecenami", "onepiecesanji", "eldenringtarnished", "eldenringmaiden", "eldenringvalkyrie", "eldenringwitch", "sevmdr", "sevmanager", "sevwellness", "sevsecurity", "offdwight", "offjim", "offmichael", "offpam", "peakytommy", "peakyarthur", "peakypolly", "peakybarman", "madmdon", "madmroger", "madmjoan", "madmpeggy", "squidplayer", "squidguard", "squidfrontman", "squidvip", "boyshomelander", "boysstarlight", "boysbutcher", "boysexec", "falloutvaultsuit", "falloutranger", "falloutraider", "falloutscientist", "rickcoat", "rickmorty", "ricksummer", "rickbirdperson", "mirrorsuit", "mirrorjumpsuit", "mirrortech", "mirrorgown", "westcowboy", "westhost", "westtech", "westgunslinger", "bttfmarty", "bttfdoc", "bttffuture", "bttfsodajerk", "jpranger", "jpscientist", "jpvisitor", "jpnedry", "wicksuit", "wickconciergeoutfit", "wicksommelier", "wickadjudicator", "kingsmangalahad", "kingsmanmerlin", "kingsmantailor", "kingsmanagent", "wonkawonka", "wonkaoompa", "wonkacharlie", "wonkaguide", "wesgustave", "weszero", "wesagatha", "wesguest", "mibsuit", "mibzed", "mibalien", "mibworm", "tronblue", "tronorange", "tronflynn", "tronsiren", "dbzgoku", "dbzvegeta", "dbzpiccolo", "dbzstudent", "amongusred", "amonguscyan", "amongusyellow", "amongusimposter", "tokyohappi", "tokyokimono", "tokyosalaryman", "tokyostreet", "egyptking", "egyptpriest", "egyptqueen", "egyptworker", "vikingwarrior", "vikingmaiden", "vikingjarl", "vikingseer", "aztecjaguarw", "azteceaglew", "aztecpriest", "aztecnoble", "obsastronomer", "obsprofessor", "obsastronaut", "obsstargazer", "tikibartender", "tikisurfer", "tikiluau", "tikitourist", "dinerwaitress", "dinercook", "dinergreaser", "dinerpoodle", "courtjudge", "courtlawyer", "courtbailiff", "courtclerk", "artsmock", "artberet", "artsculptor", "artstudent", "aquariumwetsuit", "aquariumkeeper", "aquariumbiologist", "aquariumscuba", "circusringmaster", "circusclown", "circusacrobat", "circusstrongman", "museumcurator", "museumpaleo", "museumdocent", "museumprofessor", "northpolesanta", "northpoleelf", "northpolemrsclaus", "northpolesweater", "soundstagedirector", "soundstagestarlet", "soundstagecrew", "soundstagetux", "cultrobe", "cultleader", "cultelder", "cultinitiate", "esportsjersey", "esportshoodie", "esportscaster", "esportspro", "cavemanfur", "cavemanhunter", "cavemanshaman", "cavemanchief", "lodgerobe", "lodgemaster", "lodgeapron", "lodgeguard", "prepperhazmat", "prepperfatigues", "prepperbugout", "prepperranger", "paradisepresident", "paradisesecret", "paradiseaide", "paradisehazmat", "startrekgold", "startrekblue", "startrekred", "startrekcaptain", "gentlementweedsuit", "gentlemengamekeeper", "gentlemengangster", "gentlementracksuit"],
        "colour": ["auto", "orange", "red", "pink", "purple", "blue", "teal", "green", "yellow", "grey", "black", "white"],
        "desk": ["auto", "plant", "papers", "lamp", "cactus", "duck", "trophy", "photo", "mug",
                 "helmet", "rocket", "terrarium", "quill", "candelabra", "goblet", "rum", "compass", "skull", "snowglobe", "thermos",
                 "penguin", "potions", "egg", "spellbook", "controller", "energy", "headset", "circuit", "pizza", "coffeepot",
                 "dumbbell", "shaker", "kettlebell", "coconut", "pineapple", "tikimug", "enamel", "lantern", "pinecone",
                 "jackolantern", "candles", "candy", "pumpkinball", "batfinger", "basketball",
                 "whistle", "stopwatch", "sneaker", "foamfinger", "cup", "clipboard", "cocoa", "teapot", "roses", "chocolates", "redenvelopes", "tangerines", "nachos", "latte", "champagne", "noisemaker", "floatmug", "floatplant", "lavalamp", "gumball", "hourglass", "drinkingbird", "typewriter", "antfarm", "plasmaball", "toaster", "flytrap", "cassette", "dnamodel", "dicetower", "eggtimer", "sunflower", "sourdough", "knitbasket", "rotaryphone", "addingmachine", "solderstation", "instantcam", "pinwheel", "sparklamp", "claudemug", "clawdplush", "necrotome", "tentacleink", "greenidol", "eldersignstand", "specimenjar", "trapezohedron", "whisperconch", "hardwallet", "gmmug", "tickercube", "coinstand", "moonshot", "coinpile", "bullbobble", "noodlecup", "hodlkit", "emptywallet", "cantower", "pizzastack", "coinlamp", "matchball", "psgscarf", "eiffelmini", "lakerstrophy", "celticsball", "warriorsfinger", "knickssneaker", "bullsbobble", "heatpennant", "spurstrophy", "thunderfinger", "wolvesbobble", "nuggetspennant", "hawkssneaker", "netsball", "hornetsbobble", "cavstrophy", "mavsfinger", "pistonspennant", "rocketssneaker", "pacersball", "clippersbobble", "grizzliesfinger", "buckstrophy", "pelicanspennant", "magicsneaker", "sixersbobble", "sunsball", "blazersfinger", "kingstrophy", "raptorspennant", "jazzsneaker", "wizardsball", "marvel1tesseract", "marvel1minimjolnir", "marvel1webshooter", "marvel1spideybobble", "marvel1hulkfist", "marvel1antjar", "marvel1shawarma", "marvel1compass", "marvel2walkman", "marvel2eye", "marvel2orb", "marvel2scepter", "marvel2pantherbust", "marvel2cloak", "marvel2beads", "marvel2tvaclock", "ironmanreactor", "ironmanhelmet", "ironmangauntlet", "ironmanburger", "ironmanshades", "ironmanphone", "ironmanscraps", "ironmandonut", "batcavebatarang", "batcavecowl", "batcavetea", "batcavebelt", "batcavephone", "batcavegrapple", "batcaveteeth", "batcaveumbrella", "dcmixlanternbattery", "dcmixkryptonite", "dcmixtiara", "dcmixglasses", "dcmixmotherbox", "dcmixmallet", "dcmixfortress", "dcmixflashring", "pokemonball", "pokemongameboy", "pokemonegg", "pokemonpotion", "pokemoncards", "pokemonmagikarp", "pokemonjiggly", "pokemonstones", "lolsnax", "lolward", "lolpotion", "lolcrystal", "lolteemo", "lolchest", "lolblitz", "lolmouse", "wowhearth", "wowmana", "wowgold", "wowale", "wowfrostmourne", "wowloot", "wowscroll", "wowfel", "gtacash", "gtaphone", "gtasprunk", "gtadice", "gtatoycar", "gtaradio", "gtaburger", "gtaheli", "destinyengram", "destinyglimmer", "destinygjallarhorn", "destinyshell", "destinysparrow", "destinyhelmet", "destinymote", "destinycoin", "starwarsxwing", "starwarsvaderbust", "starwarssaber", "starwarsholocron", "starwarsfalcon", "starwarstrooper", "starwarsbluemilk", "girly1lipstick", "girly1perfume", "girly1compact", "girly1phone", "girly1polish", "girly1heel", "girly1tiara", "girly1cupcake", "girly2boba", "girly2milk", "girly2plush", "girly2pencils", "girly2macarons", "girly2mushroom", "girly2headphones", "girly2onigiri", "narcoscash", "narcosbrick", "narcosphone", "narcosscale", "narcoscigar", "narcosrum", "narcospager", "narcoscoffee", "apple1iphone", "apple1airpods", "apple1watch", "apple1homepod", "apple1mouse", "apple1ipad", "apple1vision", "apple1apple", "apple2floppies", "apple2mouse", "apple2newton", "apple2imac", "apple2mug", "apple2ipod", "apple2stickies", "apple2toaster", "starwarsporg", "jewishchallah", "jewishkiddush", "jewishdreidel", "jewishpomegranate", "jewishhoney", "jewishtzedakah", "jewishspicebox", "jewishsufganiyot", "muslimtea", "muslimdates", "muslimqibla", "muslimbeads", "muslimdallah", "muslimlamp", "muslimincense", "muslimrosewater", "christianbible", "christiannativity", "christianlily", "christiancandle", "christianrosary", "christianeggs", "christianstar", "christianbuns", "buddhistbowl", "buddhistincense", "buddhistbonsai", "buddhistlotus", "buddhisttea", "buddhistzentray", "buddhistbutterlamp", "buddhistcairn", "volcanobutton", "volcanophone", "volcanoglobe", "volcanoseismo", "volcanoobsidian", "volcanomagma", "volcanoplans", "volcanokey", "kitchenmise", "kitchenboard", "kitchensauce", "kitchenbell", "kitchenplate", "kitchentimer", "kitchensalt", "kitchenwhisk", "hospitalstetho", "hospitalpills", "hospitalthermo", "hospitalcoffee", "hospitalheart", "hospitalpager", "hospitalbandage", "hospitalhammer", "noirfedora", "noirmagnifier", "noirbadge", "noirdonut", "noircasefile", "noircuffs", "noirreel", "noirradio", "madlabflask", "madlabtubes", "madlabgoggles", "madlabbattery", "madlabbulb", "madlabpetri", "madlabhand", "madlabmagnet", "casinochips", "casinodice", "casinodeck", "casinomartini", "casinoroulettemini", "casinoslotmini", "casinohorseshoe", "casinomoney", "orientticket", "orientteacup", "orientwatch", "orienttelegram", "orientlantern", "orientmodel", "orientpassport", "orientcamera", "halloweendxskull", "halloweendxapple", "halloweendxbowl", "halloweendxcrystal", "halloweendxtarot", "halloweendxgourds", "halloweendxmusicbox", "halloweendxjar", "catcafelatte", "catcafecookie", "catcafebox", "catcafemug", "catcafetreats", "catcafenapper", "catcafetoy", "catcafeteapot", "spiderversespray", "spiderversephones", "spiderverseshooter", "spiderversecomics", "spiderversesneaker", "spiderversewatch", "spiderversecollider", "spiderversepizza", "xmenclaws", "xmenvisor", "xmencards", "xmenchess", "xmenmagneto", "xmendogtags", "xmenstorm", "xmenflask", "f1car", "f1wheel", "f1tyre", "f1champagne", "f1headset", "f1cap", "f1wheelgun", "f1flag", "ufcmitts", "ufcmouthguard", "ufcwraps", "ufcbottle", "ufcbucket", "ufcicepack", "ufcscorecard", "ufccutman", "nyloftbagel", "nyloftcoffee", "nyloftslice", "nylofttaxi", "nyloftsucculent", "nyloftcard", "nyloftpretzel", "nyloftcamera", "pariscafecroissant", "pariscafeespresso", "pariscafemacarons", "pariscafebaguette", "pariscafewine", "pariscafecheese", "pariscaferose", "pariscafepaper", "muertossugarskull", "muertospan", "muertoscandle", "muertosmarigolds", "muertoscopal", "muertoschocolate", "muertosmaracas", "muertosfigurine", "submarinedeepjar", "submarinedeeprov", "submarinedeepshell", "submarinedeepthermos", "submarinedeepcoral", "submarinedeeplog", "submarinedeepradio", "submarinedeeppearl", "arcticbasecore", "arcticbasecocoa", "arcticbasemittens", "arcticbasethermo", "arcticbasebeacon", "arcticbasegoggles", "arcticbasecrystal", "arcticbasestove", "sakuramochi", "sakuradango", "sakuramatcha", "sakurabento", "sakurasprig", "sakurasoda", "sakuraonigiri", "sakurasketch", "potterwand", "pottersnitch", "pottersortinghat", "pottermonsterbook", "potterhowler", "potterchocofrog", "pottertimeturner", "potterprophet", "lotrring", "lotrpipe", "lotrlembas", "lotrphial", "lotrredbook", "lotrpalantir", "lotrmushrooms", "lotrhorn", "breakingbadhat", "breakingbadcrystal", "breakingbadflask", "breakingbadmoney", "breakingbadpollosbox", "breakingbadbear", "breakingbadphone", "breakingbadrv", "strangerwaffles", "strangerwalkie", "strangerdice", "strangercompass", "strangerdemogorgon", "strangerwalkman", "strangersundae", "strangernailbat", "goteggs", "gotgoblet", "gotdagger", "gotthrone", "gotpin", "gotscroll", "gotcoin", "gotdragonglass", "simpsonsdonut", "simpsonsduffcan", "simpsonsrod", "simpsonssquishee", "simpsonssax", "simpsonsslingshot", "simpsonsbobble", "simpsonspacifier", "bondmartini", "bondwalther", "bondwatch", "bondcards", "bondpen", "bondbriefcase", "bondgoldbar", "bondteeth", "dunespice", "dunethumper", "dunegombox", "duneliterjon", "dunepalm", "duneshield", "dunesandtimer", "dunecoffee", "onepiecehat", "onepiecemeat", "onepiecelogpose", "onepiecedevilfruit", "onepiecetangerines", "onepiecesake", "onepiececola", "onepieceponeglyph", "eldenringrune", "eldenringflasks", "eldenringgrace", "eldenringkey", "eldenringbell", "eldenringsoapstone", "eldenringjar", "eldenringlantern", "sevegg", "sevhandbook", "sevfingertrap", "sevwaffle", "sevcoffee", "sevmelon", "sevfan", "sevpencup", "offbobblehead", "offbossmug", "offjello", "offphone", "offteapot", "offdonuts", "offpaperstack", "offpretzel", "peakywhisky", "peakyashtray", "peakycash", "peakyledger", "peakywatch", "peakypint", "peakygramophone", "peakynewspaper", "madmoldfashioned", "madmtypewriter", "madmrotary", "madmashtray", "madmrolodex", "madmcigbox", "madmcoffee", "madmprojector", "squiddalgona", "squidtin", "squidddakji", "squidmarbles", "squidnumber", "squidbread", "squidjar", "squidmask", "boysmilk", "boyscoffee", "boyscape", "boysmic", "boysstatuette", "boyscomic", "boysphone", "boysenergy", "falloutnuka", "falloutpipboy", "falloutbobblehead", "falloutstimpak", "falloutcaps", "falloutlunchbox", "falloutteddy", "falloutradaway", "rickportalgun", "rickflask", "rickplumbus", "rickbutterbot", "rickraygun", "rickcrystal", "rickhipflask", "rickseed", "mirrorphone", "mirrorcookie", "mirrorlens", "mirrortablet", "mirrorspeaker", "mirrorvr", "mirrorearpiece", "mirrorcube", "westhat", "westrevolver", "westwhiskey", "westmazecoin", "westtablet", "westhand", "westcards", "westspur", "bttffluxcap", "bttfhoverboard", "bttfmrfusion", "bttfspeedo", "bttfplutonium", "bttfalmanac", "bttfmilkshake", "bttfwalkman", "jpamber", "jpdnavial", "jpjeep", "jpgoggles", "jpraptoregg", "jpcanister", "jpflare", "jptremorcup", "wickcoin", "wickmarker", "wickwhiskey", "wickphone", "wickrose", "wickdogbowl", "wickcoinstack", "wickcufflinks", "kingsmanumbrella", "kingsmanglasses", "kingsmanoxford", "kingsmansignet", "kingsmanpen", "kingsmanlighter", "kingsmanwhisky", "kingsmantape", "wonkabar", "wonkaticket", "wonkagobstopper", "wonkalollipop", "wonkafizzy", "wonkacane", "wonkahat", "wonkagum", "wesbell", "weskey", "wescake", "wesradio", "wesphone", "wesbook", "wesbinoculars", "wesperfume", "mibneuralyzer", "mibshades", "mibbadge", "mibbugjar", "mibgalaxy", "mibcoffee", "mibalienplant", "mibglobe", "tronidentity", "troncycle", "tronbaton", "tronenergy", "tronchip", "tronhelmet", "troncube", "troncoin", "dbzdragonball", "dbzcapsule", "dbzsenzu", "dbzscouter", "dbzradar", "dbztrophy", "dbznoodles", "dbzgifold", "amonguscrewmate", "amongusbutton", "amongustaskpad", "amonguswires", "amongusfuel", "amongusvisor", "amongussnack", "amongusscan", "tokyoramen", "tokyosake", "tokyomaneki", "tokyogacha", "tokyobonsai", "tokyosushi", "tokyoteapot", "tokyodaruma", "egyptscroll", "egyptcanopic", "egyptbust", "egyptscales", "egyptgoldcat", "egyptamulet", "egyptincense", "egyptsenet", "vikingtankard", "vikinghelm", "vikingaxe", "vikingrunecast", "vikingmead", "vikinglongship", "vikingtorc", "vikingfeast", "aztecskull", "aztecgoldidol", "aztecobsidian", "azteccacao", "aztecpot", "azteccalendar", "aztecfeather", "aztecmaize", "obsglobe", "obsdesktelescope", "obsastrolabe", "obsmeteorite", "obsstarchart", "obssaturn", "obscompass", "obsnotebook", "tikicoconut", "tikipineapple", "tikiukulele", "tikishell", "tikilei", "tikibananas", "tikibottle", "tikivolcano", "dinermilkshake", "dinerburger", "dinerfries", "dinercondiments", "dinercoffee", "dinerselector", "dinernapkins", "dinerdonut", "courtgavel", "courtscalesmini", "courtlawbooks", "courtinkwell", "courtnameplate", "courtcoffee", "courtbadge", "courtstamp", "artpalettemini", "artbrushes", "artpaints", "artcanvasmini", "artclay", "artpencils", "artink", "artwater", "aquariumbowl", "aquariumcoralmini", "aquariumnautilus", "aquariumdiver", "aquariumanemone", "aquariumseaglass", "aquariumpearl", "aquariumsub", "circuspopcorn", "circustophat", "circusballoons", "circuscotton", "circuspins", "circusticket", "circusjackbox", "circuscandy", "museumglobe", "museumskull", "museummicroscope", "museumammonite", "museumjar", "museummagnifier", "museummeteorite", "museumtrilo", "northpolegift", "northpolecocoa", "northpolecookies", "northpolesnowglobe", "northpolesoldier", "northpolebell", "northpolecandycane", "northpoletrain", "soundstageclapper", "soundstagemegaphone", "soundstageaward", "soundstagescript", "soundstagespotlight", "soundstagecamera", "soundstagereelcan", "soundstagecoffee", "cultcandle", "cultincense", "cultbowl", "cultcrystal", "cultchalice", "cultbell", "cultpamphlet", "cultflower", "esportskeyboard", "esportsheadset", "esportsmouse", "esportsenergy", "esportscontroller", "esportsfigure", "esportstrophy", "esportsmic", "cavemanclub", "cavemanbone", "cavemanegg", "cavemanflint", "cavemanskull", "cavemantablet", "cavemancrystal", "cavemanmeat", "lodgecandelabra", "lodgeseal", "lodgescroll", "lodgechalice", "lodgegavel", "lodgekey", "lodgehourglass", "lodgecompass", "prepperrations", "preppergasmask", "prepperradio", "prepperlantern", "preppergeiger", "prepperfirstaid", "prepperjerrycan", "preppermultitool", "paradiseflagstand", "paradisephone", "paradisenameplate", "paradisebigbutton", "paradisefolders", "paradiseintercom", "paradiseeagle", "paradiselamp", "startrektricorder", "startrekphaser", "startrekpadd", "startrekteacup", "startrekmodel", "startrekdilithium", "startrekcombadge", "startrekgauge", "gentlemendecanter", "gentlemencigars", "gentlemenpipe", "gentlemenbrick", "gentlemenfoxmask", "gentlemenledger", "gentlemenseal", "gentlemenshells", "grokmug", "grokrocket", "grokorb", "groktowel", "grokplush", "grokbotqueue", "grokbotantenna", "none"],
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
    /// How far along the wall a piece can be, in the room's pixels: the back
    /// wall, and held upright, the lounge's own wall after it, 224 on.
    static let wallLength = 448

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
        /// own workspace or thing, and the outfit and colour it gives
        /// whoever sits at it, over their role's and the dress code's but
        /// not their own; "auto" for each otherwise, and on what isn't a desk.
        var workspace = "auto"
        var monitor = "auto"
        var finish = "auto"
        var chair = "auto"
        var thing = "auto"
        var outfit = "auto"
        var colour = "auto"
    }

    /// What can be arranged: each desk, which moves and can be put away,
    /// and the lounge's pieces, which can grow, and be put away but for the
    /// red button and the mic, which are controls; and the door and the
    /// elevator, which only move along the back wall.
    static let pieces: [String: (grows: Bool, hides: Bool)] = {
        var pieces = Dictionary(uniqueKeysWithValues: (1...9).map { ("desk\($0)", (grows: false, hides: true)) })
        for part in ["wall", "corner", "couch", "pong", "plant"] { pieces[part] = (true, true) }
        for part in ["button", "mic"] { pieces[part] = (true, false) }
        // The lounge's window, which only moves along its wall
        pieces["window"] = (false, true)
        for part in ["door", "elevator"] { pieces[part] = (false, false) }
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
                                          thing: own("thing", (TeamMember.choices["desk"] ?? []).filter { $0 != "auto" }),
                                          outfit: own("outfit", TeamMember.choices["outfit"] ?? []),
                                          colour: own("colour", TeamMember.choices["colour"] ?? []))
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
        private enum Keys: String, CodingKey { case x, y, size, off, workspace, monitor, finish, chair, thing, outfit, colour }
        init(from decoder: Decoder) throws {
            guard let c = try? decoder.container(keyedBy: Keys.self) else {
                piece = nil
                return
            }
            func number(_ key: Keys) -> Double? { (try? c.decodeIfPresent(Double.self, forKey: key)) ?? nil }
            var texts: [String: String] = [:]
            for key in [Keys.workspace, .monitor, .finish, .chair, .thing, .outfit, .colour] {
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
