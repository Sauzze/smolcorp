package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
)

// Layout is how the phone's deck is laid out, as set up on the setup page: how
// many session keys, whether the office shows in their place and how it's
// furnished, whether the dial and stick show, and the rows of buttons under
// them, as the Mac app's Layout.swift. It is checked before it's kept:
// the phone draws it, and its buttons type into sessions.
type Layout struct {
	Keys KeyCount `json:"keys"`
	// The Agent Office in place of the keys: a desk for each, in the same order.
	Office bool        `json:"office"`
	Room   Room        `json:"room"`
	Dial   bool        `json:"dial"`
	Stick  bool        `json:"stick"`
	Rows   []ButtonRow `json:"rows"`
}

// KeyCount is how many session keys the deck shows: Auto, one for each
// running session, or always 3, 6 or 9. In JSON, Auto is "auto".
type KeyCount int

const (
	Auto KeyCount = 0
	// The most keys there can be, in a 3×3.
	mostKeys = 9
)

var fixedKeyCounts = []int{3, 6, 9}

func (k KeyCount) MarshalJSON() ([]byte, error) {
	if k == Auto {
		return []byte(`"auto"`), nil
	}
	return json.Marshal(int(k))
}

// UnmarshalJSON reads anything but a fixed count as Auto.
func (k *KeyCount) UnmarshalJSON(data []byte) error {
	var n int
	if json.Unmarshal(data, &n) == nil && slices.Contains(fixedKeyCounts, n) {
		*k = KeyCount(n)
	} else {
		*k = Auto
	}
	return nil
}

// Room is how the office is furnished: what's talked into, what
// each of the back wall's five places holds, the floor, the walls' paint, the
// lounge's carpet and the piece in its corner, whether the sky keeps the
// computer's time or it's always day, its pets, whether the windows follow the
// seasons, how loud it is, the desks' finish, what everyone works at (a desk,
// a tent, a throne…), the monitors, the door, what drifts over the room and the chairs' colour, how many of its
// own staff hang about, the style it's in (Yours, or one that furnishes it
// for you), your team, the dress code, and where everything goes, as the Mac
// app's Room. office.js draws it, with the same choices. Anything unknown, or of the wrong kind, is
// as standard: it only changes how the room looks.
type Room struct {
	Mic    string   `json:"mic"`
	Wave   string   `json:"wave"` // how the mic's sound waves look while it is held: the red dotted arcs ("classic") or another style
	// Button is the emergency button: "nuclear" is the red dome; the others
	// each go off their own way.
	Button string   `json:"button"`
	Wall   []string `json:"wall"`
	Floor  string   `json:"floor"`
	Paint  string   `json:"paint"`
	Lounge string   `json:"lounge"`
	// LoungePaint is the lounge's wall: "auto", as the office's walls, or a
	// paint of its own.
	LoungePaint string `json:"loungePaint"`
	Corner      string `json:"corner"`
	Sky         string `json:"sky"`
	// Pet is the office's pets, as many as petsMost, each once, in the order
	// they were picked. In a style, none picked is the style's own.
	Pet       []string `json:"pet"`
	Season    string   `json:"season"`
	Sound     string   `json:"sound"`
	Ambience  string   `json:"ambience"`  // the recorded keyboards, and talk at a desk, under the rest of the sound
	SoundWhen string   `json:"soundWhen"` // whether the sound carries on with the office behind other windows ("always"), or only while it's in front ("front")
	Desk      string   `json:"desk"`
	Workspace string   `json:"workspace"`
	Monitor   string   `json:"monitor"`
	Door      string   `json:"door"`      // the door onto the corridor: as it comes, haunted, with a wreath, a love-letter door, painted for the Lunar New Year, a locker room's, or a saloon's swinging half-doors
	Drift     string   `json:"drift"`     // what drifts, floats or crosses the room now and then: nothing, snow, hearts, the Lunar New Year's dragon dance, or confetti
	Fireworks string   `json:"fireworks"` // what the windows show after dark: nothing, fireworks round midnight, or a burst now and then every night
	Chair     string   `json:"chair"`
	Staff     string   `json:"staff"`
	Arrivals  string   `json:"arrivals"` // how a new session comes in and an ended one goes: with a coffee or a box ("animated"), or just walking ("plain")
	Calm      string   `json:"calm"`     // a calm office ("on"): nobody wanders off, the pets rest, nothing drifts by, and it's drawn less often
	Preset    string   `json:"preset"`
	// How each project's agent looks, by the project's name.
	Team map[string]TeamMember `json:"team"`
	// What agents wear by their model's size and by agent. After the team, so
	// it's written in the same place as the Mac app's Room has it.
	Dress Dress `json:"dress"`
	// Where each piece of the room is, as arranged on the setup page.
	Plan Plan `json:"plan"`
}

// TeamMember is how one project's agent looks in the office: skin, hair, a
// hairstyle, something extra such as glasses or a cap, an outfit and its
// colour, what's on its desk, what it works at, and a nickname for it. "auto"
// is as the project's name picks it, the clothes as its model's, and what it
// works at as everyone does, as the Mac app's TeamMember.
type TeamMember struct {
	Skin      string `json:"skin"`
	Hair      string `json:"hair"`
	Hairstyle string `json:"hairstyle"`
	Extra     string `json:"extra"`
	Outfit    string `json:"outfit"`
	Colour    string `json:"colour"`
	Desk      string `json:"desk"`
	Workspace string `json:"workspace"`
	Name      string `json:"name"`
}

const (
	// The most projects a team remembers, and the most a nickname can be, in
	// characters.
	teamLimit     = 50
	nicknameLimit = 16
)

// Each part of a team member's choices; the first is the standard one.
var teamChoices = map[string][]string{
	"skin": {"auto", "porcelain", "fair", "light", "tan", "olive", "brown", "deep", "ebony", "cream"},
	"hair": {"auto", "black", "espresso", "brown", "chestnut", "auburn", "ginger", "blond", "platinum", "grey", "white",
		"pink", "blue", "green", "purple", "orange"},
	"hairstyle": {"auto", "short", "long", "spiky", "bun", "curly", "ponytail", "afro", "mohawk", "buzz", "bald"},
	"extra":     {"none", "glasses", "sunglasses", "headphones", "cap", "beanie", "beard", "moustache", "bow"},
	"outfit": {"auto", "tee", "hoodie", "collar", "sweater", "jacket", "suit", "labcoat", "polo", "cardigan", "hawaiian",
		"tracksuit", "overalls", "waistcoat",
		// A style's own, and each model family's
		"astronaut", "flightsuit", "pirate", "captain", "parka", "explorer", "robe", "knight", "wizard", "gamer",
		"founder", "gymkit", "safari", "flannel", "hire", "claude", "gpt", "gemini", "grok", "qwen", "llama", "mistral",
		"deepseek", "kimi", "glm", "phi", "witch", "skeleton", "mummy", "refskeleton", "mummyjersey",
		"jersey", "warmup", "referee", "coachpolo", "blazer", "werewolf", "santa", "xmasjumper", "beanie", "cableknit", "redsuit", "heartsweater", "tangsuit", "playoffjersey", "superfan", "techwear", "submariner", "barista", "cowboy", "partyhat", "sequin",
		"artdirector", "lawyer", "therapist", "plumber", "gmexec", "gmcoach", "gmtycoon",
		"detective", "hardhat", "librarian", "janitor", "firefighter", "pilot", "postman", "accountant", "marketer", "securityguard", "chef", "spacesuit", "kimono", "cultistrobe", "investigator", "deepone", "professor", "divingsuit", "fleecevest", "bullparty", "blanketcape", "denhoodie", "homejersey", "lakersjersey", "celticsjersey", "warriorsjersey", "knicksjersey", "bullsjersey", "heatjersey", "spursjersey", "thunderjersey", "wolvesjersey", "nuggetsjersey", "hawksjersey", "netsjersey", "hornetsjersey", "cavsjersey", "mavsjersey", "pistonsjersey", "rocketsjersey", "pacersjersey", "clippersjersey", "grizzliesjersey", "bucksjersey", "pelicansjersey", "magicjersey", "sixersjersey", "sunsjersey", "blazersjersey", "kingsjersey", "raptorsjersey", "jazzjersey", "wizardsjersey", "marvel1cap", "marvel1spidey", "marvel1thor", "marvel1widow", "marvel2starlord", "marvel2strange", "marvel2panther", "marvel2loki", "ironmanmk3", "ironmanmk2", "ironmanwarmachine", "batcavebatsuit", "batcaverobin", "batcavealfred", "batcavecatwoman", "dcmixsuperman", "dcmixwonder", "dcmixflash", "dcmixgreenlantern", "pokemontrainer", "pokemonrocket", "pokemonnurse", "pokemonoak", "loljinx", "lolahri", "lolgaren", "lollux", "wowpaladin", "wowgrunt", "wowmage", "wowelf", "gtatommy", "gtacj", "gtacop", "gtaheist", "destinyhunterwear", "destinytitanwear", "destinywarlockwear", "destinyexo", "starwarsjedi", "starwarsvader", "starwarsstormtrooper", "starwarsprincess", "girly1gingham", "girly1cowgirl", "girly1tracksuit", "girly1gown", "girly2kittyhood", "girly2bunnyhood", "girly2sailor", "girly2overalls", "narcosboss", "narcosdea", "narcosmiami", "narcoshawaiian", "apple1turtleneck", "apple1genius", "apple1headset", "apple1hoodie", "apple2bowtie", "apple2woz", "apple2rainbowtee", "apple2runner", "jewishkippah", "jewishtichel", "jewishknit", "jewishdress", "muslimhijab", "muslimabaya", "muslimthobe", "muslimkurta", "christianpendant", "christianchoir", "christiansuit", "christianlace", "buddhistrobe", "buddhistmaroonrobe", "buddhistlinen", "buddhistmala", "ironmantony2", "ironmantony2b", "ironmantony2c", "ironmanmk1", "ironmanmk4", "ironmanmk5", "ironmanmk6", "ironmanmk7", "ironmanmk15", "ironmanmk16", "ironmanmk17", "ironmanmk33", "ironmanmk38", "ironmanmk42", "ironmanmk43", "ironmanmk45", "ironmanmk46", "ironmanmk47", "ironmanmk50", "ironmanmk85", "ironmanpatriot", "ironmanironspider", "ironmanrescue", "volcanojumpsuit", "volcanovillain", "volcanoheatsuit", "volcanoguard", "kitchenchef", "kitchensous", "kitchenpartie", "kitchencommis", "hospitalscrubs", "hospitaldoctor", "hospitalnurse", "hospitalsurgeon", "noirtrench", "noiruniform", "noirsuspenders", "noirreporter", "madlabcoat", "madlabapron", "madlabassistant", "madlabhazmat", "casinodealer", "casinotux", "casinohighroller", "casinoshowman", "orientconductor", "orientattendant", "orienttraveller", "orientpearls", "halloweendxvampire", "halloweendxplague", "halloweendxscarecrow", "halloweendxspider", "catcafebarista", "catcafeears", "catcafejumper", "catcafeoveralls", "spiderversemiles", "spiderversegwensuit", "spiderversepeterb", "spiderversepunk", "xmenwolverine", "xmencyclops", "xmengambit", "xmenstormsuit", "f1racesuit", "f1silversuit", "f1pitcrew", "f1teampolo", "ufcrashguard", "ufcreferee", "ufccornerhoodie", "ufcwalkout", "nyloftpeacoat", "nyloftflannel", "nyloftturtleneck", "nyloftleather", "pariscafemariniere", "pariscafegarcon", "pariscafetrench", "pariscafepearls", "muertoscatrinadress", "muertoscharro", "muertosblouse", "muertosbones", "submarinedeepcoverall", "submarinedeepwetsuit", "submarinedeeplabcoat", "submarinedeepsweater", "arcticbaseparka", "arcticbasefleece", "arcticbasecoldsuit", "arcticbaseknit", "sakurayukata", "sakuracardigan", "sakurasailor", "sakuraflorist", "pottergryffindor", "potterslytherin", "potterquidditch", "potterheadmaster", "lotrgandalf", "lotrhobbit", "lotrelf", "lotrranger", "breakingbadhazmat", "breakingbadheisenberg", "breakingbadjesse", "breakingbadsaul", "strangereleven", "strangerhellfiretee", "strangerscoopsuniform", "strangerhopper", "gotnightswatch", "gotlannisterred", "gotdaenerys", "gotwhitewalker", "simpsonshomer", "simpsonsmarge", "simpsonsbart", "simpsonslisa", "bondtux", "bondwhitejacket", "bondmoneypenny", "bondoddjob", "dunestillsuit", "dunefremen", "dunebenegesserit", "duneharkonnenblack", "onepieceluffy", "onepiecezoro", "onepiecenami", "onepiecesanji", "eldenringtarnished", "eldenringmaiden", "eldenringvalkyrie", "eldenringwitch", "sevmdr", "sevmanager", "sevwellness", "sevsecurity", "offdwight", "offjim", "offmichael", "offpam", "peakytommy", "peakyarthur", "peakypolly", "peakybarman", "madmdon", "madmroger", "madmjoan", "madmpeggy", "squidplayer", "squidguard", "squidfrontman", "squidvip", "boyshomelander", "boysstarlight", "boysbutcher", "boysexec", "falloutvaultsuit", "falloutranger", "falloutraider", "falloutscientist", "rickcoat", "rickmorty", "ricksummer", "rickbirdperson", "mirrorsuit", "mirrorjumpsuit", "mirrortech", "mirrorgown", "westcowboy", "westhost", "westtech", "westgunslinger", "bttfmarty", "bttfdoc", "bttffuture", "bttfsodajerk", "jpranger", "jpscientist", "jpvisitor", "jpnedry", "wicksuit", "wickconciergeoutfit", "wicksommelier", "wickadjudicator", "kingsmangalahad", "kingsmanmerlin", "kingsmantailor", "kingsmanagent", "wonkawonka", "wonkaoompa", "wonkacharlie", "wonkaguide", "wesgustave", "weszero", "wesagatha", "wesguest", "mibsuit", "mibzed", "mibalien", "mibworm", "tronblue", "tronorange", "tronflynn", "tronsiren", "dbzgoku", "dbzvegeta", "dbzpiccolo", "dbzstudent", "amongusred", "amonguscyan", "amongusyellow", "amongusimposter", "tokyohappi", "tokyokimono", "tokyosalaryman", "tokyostreet", "egyptking", "egyptpriest", "egyptqueen", "egyptworker", "vikingwarrior", "vikingmaiden", "vikingjarl", "vikingseer", "aztecjaguarw", "azteceaglew", "aztecpriest", "aztecnoble", "obsastronomer", "obsprofessor", "obsastronaut", "obsstargazer", "tikibartender", "tikisurfer", "tikiluau", "tikitourist", "dinerwaitress", "dinercook", "dinergreaser", "dinerpoodle", "courtjudge", "courtlawyer", "courtbailiff", "courtclerk", "artsmock", "artberet", "artsculptor", "artstudent", "aquariumwetsuit", "aquariumkeeper", "aquariumbiologist", "aquariumscuba", "circusringmaster", "circusclown", "circusacrobat", "circusstrongman", "museumcurator", "museumpaleo", "museumdocent", "museumprofessor", "northpolesanta", "northpoleelf", "northpolemrsclaus", "northpolesweater", "soundstagedirector", "soundstagestarlet", "soundstagecrew", "soundstagetux", "cultrobe", "cultleader", "cultelder", "cultinitiate", "esportsjersey", "esportshoodie", "esportscaster", "esportspro", "cavemanfur", "cavemanhunter", "cavemanshaman", "cavemanchief", "lodgerobe", "lodgemaster", "lodgeapron", "lodgeguard", "prepperhazmat", "prepperfatigues", "prepperbugout", "prepperranger", "paradisepresident", "paradisesecret", "paradiseaide", "paradisehazmat", "startrekgold", "startrekblue", "startrekred", "startrekcaptain", "gentlementweedsuit", "gentlemengamekeeper", "gentlemengangster", "gentlementracksuit"},
	"colour": {"auto", "orange", "red", "pink", "purple", "blue", "teal", "green", "yellow", "grey", "black", "white"},
	"desk": {"auto", "plant", "papers", "lamp", "cactus", "duck", "trophy", "photo", "mug",
		"helmet", "rocket", "terrarium", "quill", "candelabra", "goblet", "rum", "compass", "skull", "snowglobe", "thermos", "penguin",
		"potions", "egg", "spellbook", "controller", "energy", "headset", "circuit", "pizza", "coffeepot", "dumbbell", "shaker", "kettlebell",
		"coconut", "pineapple", "tikimug", "enamel", "lantern", "pinecone", "jackolantern", "candles", "candy", "pumpkinball", "batfinger",
		"basketball", "whistle", "stopwatch", "sneaker", "foamfinger", "cup", "clipboard", "cocoa", "teapot", "roses", "chocolates", "redenvelopes", "tangerines", "nachos", "latte", "champagne", "noisemaker", "floatmug", "floatplant", "lavalamp", "gumball", "hourglass", "drinkingbird", "typewriter", "antfarm", "plasmaball", "toaster", "flytrap", "cassette", "dnamodel", "dicetower", "eggtimer", "sunflower", "sourdough", "knitbasket", "rotaryphone", "addingmachine", "solderstation", "instantcam", "pinwheel", "sparklamp", "claudemug", "clawdplush", "necrotome", "tentacleink", "greenidol", "eldersignstand", "specimenjar", "trapezohedron", "whisperconch", "hardwallet", "gmmug", "tickercube", "coinstand", "moonshot", "coinpile", "bullbobble", "noodlecup", "hodlkit", "emptywallet", "cantower", "pizzastack", "coinlamp", "matchball", "psgscarf", "eiffelmini", "lakerstrophy", "celticsball", "warriorsfinger", "knickssneaker", "bullsbobble", "heatpennant", "spurstrophy", "thunderfinger", "wolvesbobble", "nuggetspennant", "hawkssneaker", "netsball", "hornetsbobble", "cavstrophy", "mavsfinger", "pistonspennant", "rocketssneaker", "pacersball", "clippersbobble", "grizzliesfinger", "buckstrophy", "pelicanspennant", "magicsneaker", "sixersbobble", "sunsball", "blazersfinger", "kingstrophy", "raptorspennant", "jazzsneaker", "wizardsball", "marvel1tesseract", "marvel1minimjolnir", "marvel1webshooter", "marvel1spideybobble", "marvel1hulkfist", "marvel1antjar", "marvel1shawarma", "marvel1compass", "marvel2walkman", "marvel2eye", "marvel2orb", "marvel2scepter", "marvel2pantherbust", "marvel2cloak", "marvel2beads", "marvel2tvaclock", "ironmanreactor", "ironmanhelmet", "ironmangauntlet", "ironmanburger", "ironmanshades", "ironmanphone", "ironmanscraps", "ironmandonut", "batcavebatarang", "batcavecowl", "batcavetea", "batcavebelt", "batcavephone", "batcavegrapple", "batcaveteeth", "batcaveumbrella", "dcmixlanternbattery", "dcmixkryptonite", "dcmixtiara", "dcmixglasses", "dcmixmotherbox", "dcmixmallet", "dcmixfortress", "dcmixflashring", "pokemonball", "pokemongameboy", "pokemonegg", "pokemonpotion", "pokemoncards", "pokemonmagikarp", "pokemonjiggly", "pokemonstones", "lolsnax", "lolward", "lolpotion", "lolcrystal", "lolteemo", "lolchest", "lolblitz", "lolmouse", "wowhearth", "wowmana", "wowgold", "wowale", "wowfrostmourne", "wowloot", "wowscroll", "wowfel", "gtacash", "gtaphone", "gtasprunk", "gtadice", "gtatoycar", "gtaradio", "gtaburger", "gtaheli", "destinyengram", "destinyglimmer", "destinygjallarhorn", "destinyshell", "destinysparrow", "destinyhelmet", "destinymote", "destinycoin", "starwarsxwing", "starwarsvaderbust", "starwarssaber", "starwarsholocron", "starwarsfalcon", "starwarstrooper", "starwarsbluemilk", "girly1lipstick", "girly1perfume", "girly1compact", "girly1phone", "girly1polish", "girly1heel", "girly1tiara", "girly1cupcake", "girly2boba", "girly2milk", "girly2plush", "girly2pencils", "girly2macarons", "girly2mushroom", "girly2headphones", "girly2onigiri", "narcoscash", "narcosbrick", "narcosphone", "narcosscale", "narcoscigar", "narcosrum", "narcospager", "narcoscoffee", "apple1iphone", "apple1airpods", "apple1watch", "apple1homepod", "apple1mouse", "apple1ipad", "apple1vision", "apple1apple", "apple2floppies", "apple2mouse", "apple2newton", "apple2imac", "apple2mug", "apple2ipod", "apple2stickies", "apple2toaster", "starwarsporg", "jewishchallah", "jewishkiddush", "jewishdreidel", "jewishpomegranate", "jewishhoney", "jewishtzedakah", "jewishspicebox", "jewishsufganiyot", "muslimtea", "muslimdates", "muslimqibla", "muslimbeads", "muslimdallah", "muslimlamp", "muslimincense", "muslimrosewater", "christianbible", "christiannativity", "christianlily", "christiancandle", "christianrosary", "christianeggs", "christianstar", "christianbuns", "buddhistbowl", "buddhistincense", "buddhistbonsai", "buddhistlotus", "buddhisttea", "buddhistzentray", "buddhistbutterlamp", "buddhistcairn", "volcanobutton", "volcanophone", "volcanoglobe", "volcanoseismo", "volcanoobsidian", "volcanomagma", "volcanoplans", "volcanokey", "kitchenmise", "kitchenboard", "kitchensauce", "kitchenbell", "kitchenplate", "kitchentimer", "kitchensalt", "kitchenwhisk", "hospitalstetho", "hospitalpills", "hospitalthermo", "hospitalcoffee", "hospitalheart", "hospitalpager", "hospitalbandage", "hospitalhammer", "noirfedora", "noirmagnifier", "noirbadge", "noirdonut", "noircasefile", "noircuffs", "noirreel", "noirradio", "madlabflask", "madlabtubes", "madlabgoggles", "madlabbattery", "madlabbulb", "madlabpetri", "madlabhand", "madlabmagnet", "casinochips", "casinodice", "casinodeck", "casinomartini", "casinoroulettemini", "casinoslotmini", "casinohorseshoe", "casinomoney", "orientticket", "orientteacup", "orientwatch", "orienttelegram", "orientlantern", "orientmodel", "orientpassport", "orientcamera", "halloweendxskull", "halloweendxapple", "halloweendxbowl", "halloweendxcrystal", "halloweendxtarot", "halloweendxgourds", "halloweendxmusicbox", "halloweendxjar", "catcafelatte", "catcafecookie", "catcafebox", "catcafemug", "catcafetreats", "catcafenapper", "catcafetoy", "catcafeteapot", "spiderversespray", "spiderversephones", "spiderverseshooter", "spiderversecomics", "spiderversesneaker", "spiderversewatch", "spiderversecollider", "spiderversepizza", "xmenclaws", "xmenvisor", "xmencards", "xmenchess", "xmenmagneto", "xmendogtags", "xmenstorm", "xmenflask", "f1car", "f1wheel", "f1tyre", "f1champagne", "f1headset", "f1cap", "f1wheelgun", "f1flag", "ufcmitts", "ufcmouthguard", "ufcwraps", "ufcbottle", "ufcbucket", "ufcicepack", "ufcscorecard", "ufccutman", "nyloftbagel", "nyloftcoffee", "nyloftslice", "nylofttaxi", "nyloftsucculent", "nyloftcard", "nyloftpretzel", "nyloftcamera", "pariscafecroissant", "pariscafeespresso", "pariscafemacarons", "pariscafebaguette", "pariscafewine", "pariscafecheese", "pariscaferose", "pariscafepaper", "muertossugarskull", "muertospan", "muertoscandle", "muertosmarigolds", "muertoscopal", "muertoschocolate", "muertosmaracas", "muertosfigurine", "submarinedeepjar", "submarinedeeprov", "submarinedeepshell", "submarinedeepthermos", "submarinedeepcoral", "submarinedeeplog", "submarinedeepradio", "submarinedeeppearl", "arcticbasecore", "arcticbasecocoa", "arcticbasemittens", "arcticbasethermo", "arcticbasebeacon", "arcticbasegoggles", "arcticbasecrystal", "arcticbasestove", "sakuramochi", "sakuradango", "sakuramatcha", "sakurabento", "sakurasprig", "sakurasoda", "sakuraonigiri", "sakurasketch", "potterwand", "pottersnitch", "pottersortinghat", "pottermonsterbook", "potterhowler", "potterchocofrog", "pottertimeturner", "potterprophet", "lotrring", "lotrpipe", "lotrlembas", "lotrphial", "lotrredbook", "lotrpalantir", "lotrmushrooms", "lotrhorn", "breakingbadhat", "breakingbadcrystal", "breakingbadflask", "breakingbadmoney", "breakingbadpollosbox", "breakingbadbear", "breakingbadphone", "breakingbadrv", "strangerwaffles", "strangerwalkie", "strangerdice", "strangercompass", "strangerdemogorgon", "strangerwalkman", "strangersundae", "strangernailbat", "goteggs", "gotgoblet", "gotdagger", "gotthrone", "gotpin", "gotscroll", "gotcoin", "gotdragonglass", "simpsonsdonut", "simpsonsduffcan", "simpsonsrod", "simpsonssquishee", "simpsonssax", "simpsonsslingshot", "simpsonsbobble", "simpsonspacifier", "bondmartini", "bondwalther", "bondwatch", "bondcards", "bondpen", "bondbriefcase", "bondgoldbar", "bondteeth", "dunespice", "dunethumper", "dunegombox", "duneliterjon", "dunepalm", "duneshield", "dunesandtimer", "dunecoffee", "onepiecehat", "onepiecemeat", "onepiecelogpose", "onepiecedevilfruit", "onepiecetangerines", "onepiecesake", "onepiececola", "onepieceponeglyph", "eldenringrune", "eldenringflasks", "eldenringgrace", "eldenringkey", "eldenringbell", "eldenringsoapstone", "eldenringjar", "eldenringlantern", "sevegg", "sevhandbook", "sevfingertrap", "sevwaffle", "sevcoffee", "sevmelon", "sevfan", "sevpencup", "offbobblehead", "offbossmug", "offjello", "offphone", "offteapot", "offdonuts", "offpaperstack", "offpretzel", "peakywhisky", "peakyashtray", "peakycash", "peakyledger", "peakywatch", "peakypint", "peakygramophone", "peakynewspaper", "madmoldfashioned", "madmtypewriter", "madmrotary", "madmashtray", "madmrolodex", "madmcigbox", "madmcoffee", "madmprojector", "squiddalgona", "squidtin", "squidddakji", "squidmarbles", "squidnumber", "squidbread", "squidjar", "squidmask", "boysmilk", "boyscoffee", "boyscape", "boysmic", "boysstatuette", "boyscomic", "boysphone", "boysenergy", "falloutnuka", "falloutpipboy", "falloutbobblehead", "falloutstimpak", "falloutcaps", "falloutlunchbox", "falloutteddy", "falloutradaway", "rickportalgun", "rickflask", "rickplumbus", "rickbutterbot", "rickraygun", "rickcrystal", "rickhipflask", "rickseed", "mirrorphone", "mirrorcookie", "mirrorlens", "mirrortablet", "mirrorspeaker", "mirrorvr", "mirrorearpiece", "mirrorcube", "westhat", "westrevolver", "westwhiskey", "westmazecoin", "westtablet", "westhand", "westcards", "westspur", "bttffluxcap", "bttfhoverboard", "bttfmrfusion", "bttfspeedo", "bttfplutonium", "bttfalmanac", "bttfmilkshake", "bttfwalkman", "jpamber", "jpdnavial", "jpjeep", "jpgoggles", "jpraptoregg", "jpcanister", "jpflare", "jptremorcup", "wickcoin", "wickmarker", "wickwhiskey", "wickphone", "wickrose", "wickdogbowl", "wickcoinstack", "wickcufflinks", "kingsmanumbrella", "kingsmanglasses", "kingsmanoxford", "kingsmansignet", "kingsmanpen", "kingsmanlighter", "kingsmanwhisky", "kingsmantape", "wonkabar", "wonkaticket", "wonkagobstopper", "wonkalollipop", "wonkafizzy", "wonkacane", "wonkahat", "wonkagum", "wesbell", "weskey", "wescake", "wesradio", "wesphone", "wesbook", "wesbinoculars", "wesperfume", "mibneuralyzer", "mibshades", "mibbadge", "mibbugjar", "mibgalaxy", "mibcoffee", "mibalienplant", "mibglobe", "tronidentity", "troncycle", "tronbaton", "tronenergy", "tronchip", "tronhelmet", "troncube", "troncoin", "dbzdragonball", "dbzcapsule", "dbzsenzu", "dbzscouter", "dbzradar", "dbztrophy", "dbznoodles", "dbzgifold", "amonguscrewmate", "amongusbutton", "amongustaskpad", "amonguswires", "amongusfuel", "amongusvisor", "amongussnack", "amongusscan", "tokyoramen", "tokyosake", "tokyomaneki", "tokyogacha", "tokyobonsai", "tokyosushi", "tokyoteapot", "tokyodaruma", "egyptscroll", "egyptcanopic", "egyptbust", "egyptscales", "egyptgoldcat", "egyptamulet", "egyptincense", "egyptsenet", "vikingtankard", "vikinghelm", "vikingaxe", "vikingrunecast", "vikingmead", "vikinglongship", "vikingtorc", "vikingfeast", "aztecskull", "aztecgoldidol", "aztecobsidian", "azteccacao", "aztecpot", "azteccalendar", "aztecfeather", "aztecmaize", "obsglobe", "obsdesktelescope", "obsastrolabe", "obsmeteorite", "obsstarchart", "obssaturn", "obscompass", "obsnotebook", "tikicoconut", "tikipineapple", "tikiukulele", "tikishell", "tikilei", "tikibananas", "tikibottle", "tikivolcano", "dinermilkshake", "dinerburger", "dinerfries", "dinercondiments", "dinercoffee", "dinerselector", "dinernapkins", "dinerdonut", "courtgavel", "courtscalesmini", "courtlawbooks", "courtinkwell", "courtnameplate", "courtcoffee", "courtbadge", "courtstamp", "artpalettemini", "artbrushes", "artpaints", "artcanvasmini", "artclay", "artpencils", "artink", "artwater", "aquariumbowl", "aquariumcoralmini", "aquariumnautilus", "aquariumdiver", "aquariumanemone", "aquariumseaglass", "aquariumpearl", "aquariumsub", "circuspopcorn", "circustophat", "circusballoons", "circuscotton", "circuspins", "circusticket", "circusjackbox", "circuscandy", "museumglobe", "museumskull", "museummicroscope", "museumammonite", "museumjar", "museummagnifier", "museummeteorite", "museumtrilo", "northpolegift", "northpolecocoa", "northpolecookies", "northpolesnowglobe", "northpolesoldier", "northpolebell", "northpolecandycane", "northpoletrain", "soundstageclapper", "soundstagemegaphone", "soundstageaward", "soundstagescript", "soundstagespotlight", "soundstagecamera", "soundstagereelcan", "soundstagecoffee", "cultcandle", "cultincense", "cultbowl", "cultcrystal", "cultchalice", "cultbell", "cultpamphlet", "cultflower", "esportskeyboard", "esportsheadset", "esportsmouse", "esportsenergy", "esportscontroller", "esportsfigure", "esportstrophy", "esportsmic", "cavemanclub", "cavemanbone", "cavemanegg", "cavemanflint", "cavemanskull", "cavemantablet", "cavemancrystal", "cavemanmeat", "lodgecandelabra", "lodgeseal", "lodgescroll", "lodgechalice", "lodgegavel", "lodgekey", "lodgehourglass", "lodgecompass", "prepperrations", "preppergasmask", "prepperradio", "prepperlantern", "preppergeiger", "prepperfirstaid", "prepperjerrycan", "preppermultitool", "paradiseflagstand", "paradisephone", "paradisenameplate", "paradisebigbutton", "paradisefolders", "paradiseintercom", "paradiseeagle", "paradiselamp", "startrektricorder", "startrekphaser", "startrekpadd", "startrekteacup", "startrekmodel", "startrekdilithium", "startrekcombadge", "startrekgauge", "gentlemendecanter", "gentlemencigars", "gentlemenpipe", "gentlemenbrick", "gentlemenfoxmask", "gentlemenledger", "gentlemenseal", "gentlemenshells", "grokmug", "grokrocket", "grokorb", "groktowel", "grokplush", "grokbotqueue", "grokbotantenna", "none"},
	"workspace": append([]string{"auto"}, workspaces...),
}

// What an agent can work at: the office's choice for everyone, the first
// being standard, and a team member's own, which can also be "auto".
var (
	floors = []string{"wood", "tiles", "concrete", "carpet", "plating", "deck", "marble", "flagstone", "snow", "grass", "forest",
		"rubber", "epoxy", "arcade", "hauntedcourt", "court", "studio", "stationfloor", "zensand", "planks", "wetflags", "polished", "emeraldgold", "worncarpet", "cablefloor", "pitchgrass", "lakerscourt", "celticscourt", "warriorscourt", "knickscourt", "bullscourt", "heatcourt", "spurscourt", "thundercourt", "wolvescourt", "nuggetscourt", "hawkscourt", "netscourt", "hornetscourt", "cavscourt", "mavscourt", "pistonscourt", "rocketscourt", "pacerscourt", "clipperscourt", "grizzliescourt", "buckscourt", "pelicanscourt", "magiccourt", "sixerscourt", "sunscourt", "blazerscourt", "kingscourt", "raptorscourt", "jazzcourt", "wizardscourt", "marvel1tower", "marvel1stars", "marvel2vibranium", "marvel2cosmos", "ironmangarage", "ironmanhexglass", "batcavedeck", "batcaverock", "dcmixwatchtower", "dcmixthemyscira", "pokemontiles", "pokemongrass", "lolhextiles", "lolrift", "wowinn", "wowcobble", "gtavice", "gtaasphalt", "destinytower", "destinyrust", "starwarsdeck", "starwarssand", "girly1marble", "girly1shag", "girly2mint", "girly2cloud", "narcostiles", "narcosleopard", "apple1oak", "apple1stone", "apple2carpet", "apple2dither", "jewishstone", "jewishtiles", "muslimzellige", "muslimcarpet", "christianflags", "christianoak", "buddhisttatami", "buddhistslate", "volcanobasalt", "volcanogrid", "kitchenquarry", "kitchenmat", "hospitallino", "hospitallines", "noirchecker", "noirboards", "madlabdrain", "madlabplate", "casinocarpet", "casinomarble", "orientterrazzo", "orientcarpet", "halloweendxboards", "halloweendxsoil", "catcafeoak", "catcafepatch", "spiderverserooftop", "spiderversehalftone", "xmendanger", "xmenparquet", "f1tarmac", "f1chequer", "ufccanvas", "ufcmats", "nyloftboards", "nyloftrug", "pariscafetiles", "pariscafechevron", "muertostiles", "muertospetals", "submarinedeepgrate", "submarinedeepmat", "arcticbasetread", "arcticbaserug", "sakuraboards", "sakuralawn", "potterflags", "pottercommon", "lotrshire", "lotrmeduseld", "breakingbadlab", "breakingbaddesert", "strangerarcade", "strangerupside", "gotwinterfell", "gotredkeep", "simpsonsplant", "simpsonsliving", "bondmi6", "bondlair", "dunesietch", "dunesand", "onepiecedeck", "onepiecelawn", "eldenringleyndell", "eldenringlimgrave", "sevcarpet", "sevtile", "offcarpet", "offlino", "peakyboards", "peakycobble", "madmcarpet", "madmparquet", "squidfield", "squidteal", "boysmarble", "boyscarpet", "falloutvault", "falloutconcrete", "rickgarage", "rickportal", "mirrorfloor", "mirrordark", "westlab", "westsaloon", "bttfgaragefloor", "bttfdinerfloor", "jpvisitorfloor", "jpjunglefloor", "wicklobbyfloor", "wickvaultfloor", "kingsmanshopfloor", "kingsmanhqfloor", "wonkariverfloor", "wonkacandyfloor", "weslobbyfloor", "wescorridorfloor", "mibhqfloor", "mibtunnelfloor", "trongridfloor", "tronarenafloor", "dbzdojofloor", "dbzlabfloor", "amongusshipfloor", "amonguscafefloor", "tokyofloor", "tokyotatami", "egyptfloor", "egyptmosaic", "vikingfloor", "vikingstone", "aztecfloor", "aztecjungle", "obsfloor", "obscarpet", "tikifloor", "tikisand", "dinerfloor", "dinerterrazzo", "courtfloor", "courtcarpet", "artfloor", "artrug", "aquariumfloor", "aquariumglass", "circussawdust", "circusredcarpet", "museummarble", "museumparquet", "northpolesnow", "northpoleplank", "soundstagefloor", "soundstagelounge", "cultclay", "cultmat", "esportsstagefloor", "esportscarpet", "cavemanrock", "cavemanhide", "lodgechecker", "lodgecarpet", "prepperconcrete", "prepperrubber", "paradiseovalrug", "paradisebunkerfloor", "startrekdeck", "startrekpad", "gentlemenherringbone", "gentlementartan", "grokfloor", "grokrug", "grokbotfloor"}
	paints = []string{"slate", "sage", "clay", "cream", "hull", "timber", "damask", "stone", "ice", "logs", "brick", "bamboo",
		"block", "foam", "primetime", "bleachers", "stationwall", "shoji", "plankwall", "deepsea", "fintech", "greenglow", "frostblue", "ledstrip", "parcnavy", "lakerswall", "celticswall", "warriorswall", "knickswall", "bullswall", "heatwall", "spurswall", "thunderwall", "wolveswall", "nuggetswall", "hawkswall", "netswall", "hornetswall", "cavswall", "mavswall", "pistonswall", "rocketswall", "pacerswall", "clipperswall", "grizzlieswall", "buckswall", "pelicanswall", "magicwall", "sixerswall", "sunswall", "blazerswall", "kingswall", "raptorswall", "jazzwall", "wizardswall", "marvel1red", "marvel1webwall", "marvel2wakanda", "marvel2sanctum", "ironmanred", "ironmanglass", "batcavestone", "batcavemanor", "dcmixsuperblue", "dcmixlanternwall", "pokemoncenter", "pokemonroute", "lolpiltover", "loljungle", "wowstormwind", "wowhorde", "gtavicecity", "gtabrick", "destinylastcity", "destinyvoid", "starwarsimperial", "starwarsrebel", "girly1hotpink", "girly1blush", "girly2lilac", "girly2mintwall", "narcoshacienda", "narcosjungle", "apple1white", "apple1space", "apple2beige", "apple2rainbow", "jewishblue", "jewishsand", "muslimteal", "muslimplaster", "christianwhite", "christianwine", "buddhistsaffron", "buddhistmaroon", "volcanorock", "volcanosteel", "kitchentile", "kitchenbrick", "hospitalmint", "hospitalwhite", "noirplaster", "noirgreen", "madlabstone", "madlabtile", "casinored", "casinoneon", "orientmarquetry", "orientcream", "halloweendxdamask", "halloweendxcrypt", "catcafepink", "catcafemint", "spiderversegraffiti", "spiderversegwen", "xmenmansion", "xmenhangar", "f1pitwall", "f1carbon", "ufcarena", "ufcgym", "nyloftbrick", "nyloftplaster", "pariscafeboiserie", "pariscafebordeaux", "muertospink", "muertosteal", "submarinedeepwhite", "submarinedeepnavy", "arcticbasered", "arcticbasepine", "sakurablush", "sakurasky", "pottergryffindor", "potterstone", "lotrbagend", "lotrrivendell", "breakingbadsuperlab", "breakingbadstucco", "strangerwallpaper", "strangerupsidewall", "gotstone", "gotcrimson", "simpsonspink", "simpsonssky", "bondwalnut", "bondgunwall", "duneatreides", "dunerock", "onepiecesunny", "onepiecesea", "eldenringstormveil", "eldenringerdtree", "sevwall", "sevhall", "offwall", "offbreak", "peakywall", "peakybrick", "madmwall", "madmteak", "squidstairs", "squidpink", "boyswall", "boysred", "falloutwall", "falloutgrime", "rickwall", "rickalien", "mirrorwall", "mirrordarkwall", "westlabwall", "westsaloonwall", "bttfgaragewall", "bttfdinerwall", "jpconcretewall", "jpjunglewall", "wicklobbywall", "wickvaultwall", "kingsmanshopwall", "kingsmanhqwall", "wonkafactorywall", "wonkagardenwall", "weslobbywall", "wescorridorwall", "mibhqwall", "mibtunnelwall", "trongridwall", "tronorangewall", "dbzdojowall", "dbzlabwall", "amongusshipwall", "amonguscafewall", "tokyoneonwall", "tokyobrick", "egyptwall", "egyptgold", "vikingwall", "vikingrune", "aztecwall", "aztecvine", "obswall", "obspanel", "tikiwall", "tikisunset", "dinerwall", "dinerstripe", "courtwall", "courtmarble", "artwall", "artbrick", "aquariumwall", "aquariumcoral", "circusbigtop", "circusmarquee", "museumstone", "museumgallery", "northpolelog", "northpolewrap", "soundstageacoustic", "soundstagegreenroom", "cultadobe", "cultveil", "esportsled", "esportspanel", "cavemanwall", "cavemanpaintings", "lodgepanel", "lodgedrape", "prepperwall", "preppersandbag", "paradiseoval", "paradiseconcrete", "startrekbulkhead", "startrekjefferies", "gentlemenpanel", "gentlementweed", "grokwall", "grokpanel", "grokbotwall"}
)

// roomStyles are the office's styles: Yours, as you furnished it, then each that
// furnishes it for you. office.js keeps what each one is.
var roomStyles = []string{"yours", "classic", "campsite", "royal", "space", "pirates", "arctic", "lair", "gamers", "garage", "gym",
	"jungle", "neoncity", "submarine", "lofi", "saloon", "zen", "treehouse", "lovecraft", "claudeteam", "cryptooffice", "bullrun", "bearmarket", "cryptoden", "psg", "lakers", "celtics", "warriors", "knicks", "bulls", "heat", "spurs", "thunder", "wolves", "nuggets", "hawks", "nets", "hornets", "cavs", "mavs", "pistons", "rockets", "pacers", "clippers", "grizzlies", "bucks", "pelicans", "magic", "sixers", "suns", "blazers", "kings", "raptors", "jazz", "wizards", "marvel1", "marvel2", "ironman", "batcave", "dcmix", "pokemon", "lol", "wow", "gta", "destiny", "starwars", "girly1", "girly2", "narcos", "apple1", "apple2", "jewish", "muslim", "christian", "buddhist", "ironmanhall", "volcano", "kitchen", "hospital", "noir", "madlab", "casino", "orient", "halloweendx", "catcafe", "spiderverse", "xmen", "f1", "ufc", "nyloft", "pariscafe", "muertos", "submarinedeep", "arcticbase", "sakura", "potter", "lotr", "breakingbad", "stranger", "got", "simpsons", "bond", "dune", "onepiece", "eldenring", "sev", "off", "peaky", "madm", "squid", "boys", "fallout", "rick", "mirror", "west", "bttf", "jp", "wick", "kingsman", "wonka", "wes", "mib", "tron", "dbz", "amongus", "tokyo", "egypt", "viking", "aztec", "obs", "tiki", "diner", "court", "art", "aquarium", "circus", "museum", "northpole", "soundstage", "cult", "esports", "caveman", "lodge", "prepper", "paradise", "startrek", "gentlemen", "grok", "grokbots"}

var workspaces = []string{"desk", "standing", "gaming", "tent", "box", "beanbag", "igloo", "van", "cockpit", "ship", "throne",
	"greenhouse", "console", "hover", "chest", "gilded", "crates", "altar", "log", "bench", "bamboo", "rack", "drafting", "glass",
	"scorer", "anchor", "coffin", "waxdesk", "clawdesk", "gingerdesk", "workbench", "plaiddesk", "pinkdesk", "lacedesk", "lacquerdesk", "courtdesk", "cyberdesk", "subconsole", "cafetable", "barreldesk", "discodesk", "lowtable", "cratedesk", "scriptorium", "charttable", "tradingdesk", "bulldesk", "heaterdesk", "screenwall", "pitchdesk", "lakersdesk", "celticsdesk", "warriorsdesk", "knicksdesk", "bullsdesk", "heatdesk", "spursdesk", "thunderdesk", "wolvesdesk", "nuggetsdesk", "hawksdesk", "netsdesk", "hornetsdesk", "cavsdesk", "mavsdesk", "pistonsdesk", "rocketsdesk", "pacersdesk", "clippersdesk", "grizzliesdesk", "bucksdesk", "pelicansdesk", "magicdesk", "sixersdesk", "sunsdesk", "blazersdesk", "kingsdesk", "raptorsdesk", "jazzdesk", "wizardsdesk", "marvel1deck", "marvel2milano", "ironmanbench", "batcaveconsole", "dcmixplanetdesk", "pokemonballdesk", "lolhexdesk", "wowquestdesk", "gtalowrider", "destinyvanguard", "starwarsconsole", "girly1vanity", "girly2desk", "narcosdesk", "apple1table", "apple2desk", "jewishdesk", "muslimdesk", "christiandesk", "buddhistdesk", "volcanodesk", "kitchendesk", "hospitaldesk", "noirdesk", "madlabdesk", "casinodesk", "orientdesk", "halloweendxdesk", "catcafedesk", "spiderversedesk", "xmendesk", "f1desk", "ufcdesk", "nyloftdesk", "pariscafedesk", "muertosdesk", "submarinedeepdesk", "arcticbasedesk", "sakuradesk", "potterdesk", "lotrdesk", "breakingbaddesk", "strangerdesk", "gotdesk", "simpsonsconsole", "bonddesk", "dunedesk", "onepiecedesk", "eldenringdesk", "sevdesk", "offdesk", "peakydesk", "madmdesk", "squiddesk", "boysdesk", "falloutdesk", "rickdesk", "mirrordesk", "westdesk", "bttfbench", "jpcontroldesk", "wickconcierge", "kingsmancuttingtable", "wonkadesk", "wesconcierge", "mibdesk", "trondesk", "dbzdesk", "amongusdesk", "tokyodesk", "egyptdesk", "vikingdesk", "aztecdesk", "obsdesk", "tikidesk", "dinerdesk", "courtcounsel", "artdesk", "aquariumdesk", "circusdesk", "museumdesk", "northpolebench", "soundstagedesk", "cultdesk", "esportsrig", "cavemanslab", "lodgedesk", "prepperdesk", "paradiseresolute", "startrekbridge", "gentlemenpartners", "grokdesk", "grokbotpod"}

// checked is the member with anything unknown as standard, and the nickname
// on one line, cut to its limit.
func (m TeamMember) checked() TeamMember {
	choose := func(part, given string) string {
		if slices.Contains(teamChoices[part], given) {
			return given
		}
		return teamChoices[part][0]
	}
	return TeamMember{Skin: choose("skin", m.Skin), Hair: choose("hair", m.Hair), Hairstyle: choose("hairstyle", m.Hairstyle),
		Extra: choose("extra", m.Extra), Outfit: choose("outfit", m.Outfit), Colour: choose("colour", m.Colour),
		Desk: choose("desk", m.Desk), Workspace: choose("workspace", m.Workspace), Name: oneLine(m.Name, nicknameLimit)}
}

// Dress is the office's dress code: what agents wear by the size of their
// model, and by which agent they are, and rules that give a model its size by
// part of its name, as the Mac app's Dress. Every size and every agent is
// always there, "auto" when it isn't set, so office.js never finds one
// missing. Anything unknown, or of the wrong kind, is as standard, as in the
// rest of the room.
type Dress struct {
	Sizes  DressSizes  `json:"sizes"`
	Agents DressAgents `json:"agents"`
	// In the order given.
	Rules []DressRule `json:"rules"`
}

// DressSizes is what agents wear by the size of their model. A struct rather
// than a map, which would be written in alphabetical order: this keeps the
// sizes small to top.
type DressSizes struct {
	Small  Attire `json:"small"`
	Medium Attire `json:"medium"`
	Large  Attire `json:"large"`
	Top    Attire `json:"top"`
}

// DressAgents is what each agent wears, in the deck's order of agents.
type DressAgents struct {
	Claude   Attire `json:"claude"`
	Codex    Attire `json:"codex"`
	Gemini   Attire `json:"gemini"`
	OpenCode Attire `json:"opencode"`
	Grok     Attire `json:"grok"`
}

// Attire is what the dress code puts on agents: an outfit and its colour,
// from a team member's choices. "auto" is as its model's, as for a team
// member.
type Attire struct {
	Outfit string `json:"outfit"`
	Colour string `json:"colour"`
}

// DressRule dresses a model whose name has Contains in it as Size. What it
// looks for is kept on one line, and lowercased.
type DressRule struct {
	Contains string `json:"contains"`
	Size     string `json:"size"`
}

const (
	// The most rules kept, and the most a rule's part of a name can be, in
	// characters.
	dressRuleLimit     = 20
	dressContainsLimit = 32
)

var dressSizes = []string{"small", "medium", "large", "top"}

// UnmarshalJSON keeps what it knows of the dress code it's given. A dress
// code, an attire or a rule that isn't an object is as standard.
func (d *Dress) UnmarshalJSON(data []byte) error {
	object := func(data json.RawMessage) map[string]json.RawMessage {
		var o map[string]json.RawMessage
		json.Unmarshal(data, &o)
		return o
	}
	text := func(data json.RawMessage) string {
		var s string
		json.Unmarshal(data, &s)
		return s
	}
	attire := func(set map[string]json.RawMessage, name string) Attire {
		parts := object(set[name])
		return Attire{Outfit: text(parts["outfit"]), Colour: text(parts["colour"])}
	}
	raw := object(data)
	sizes, agents := object(raw["sizes"]), object(raw["agents"])
	*d = Dress{
		Sizes: DressSizes{Small: attire(sizes, "small"), Medium: attire(sizes, "medium"), Large: attire(sizes, "large"),
			Top: attire(sizes, "top")},
		Agents: DressAgents{Claude: attire(agents, string(Claude)), Codex: attire(agents, string(Codex)),
			Gemini: attire(agents, string(Gemini)), OpenCode: attire(agents, string(OpenCode)), Grok: attire(agents, string(Grok))},
	}
	var rules []json.RawMessage
	json.Unmarshal(raw["rules"], &rules)
	for _, rule := range rules {
		parts := object(rule)
		d.Rules = append(d.Rules, DressRule{Contains: text(parts["contains"]), Size: text(parts["size"])})
	}
	*d = d.checked()
	return nil
}

// checked is the dress code with anything unknown as auto, and the rules
// that name a part of a name and a size the office has, as many as it keeps.
func (d Dress) checked() Dress {
	checked := Dress{
		Sizes: DressSizes{Small: d.Sizes.Small.checked(), Medium: d.Sizes.Medium.checked(), Large: d.Sizes.Large.checked(),
			Top: d.Sizes.Top.checked()},
		Agents: DressAgents{Claude: d.Agents.Claude.checked(), Codex: d.Agents.Codex.checked(), Gemini: d.Agents.Gemini.checked(),
			OpenCode: d.Agents.OpenCode.checked(), Grok: d.Agents.Grok.checked()},
		Rules: []DressRule{},
	}
	for _, rule := range d.Rules {
		if len(checked.Rules) == dressRuleLimit {
			break
		}
		contains := oneLine(strings.ToLower(rule.Contains), dressContainsLimit)
		if contains != "" && slices.Contains(dressSizes, rule.Size) {
			checked.Rules = append(checked.Rules, DressRule{Contains: contains, Size: rule.Size})
		}
	}
	return checked
}

// checked is the attire with anything unknown as auto, by a team member's
// choices.
func (a Attire) checked() Attire {
	member := TeamMember{Outfit: a.Outfit, Colour: a.Colour}.checked()
	return Attire{Outfit: member.Outfit, Colour: member.Colour}
}

var (
	// Each part's choices; the first is the standard one.
	roomChoices = map[string][]string{
		"wave":  {"classic", "orange", "ocean", "mint", "gold", "rainbow", "rings", "dashed", "sine", "eq", "dots", "notes", "hearts", "sparks", "lightning", "speech", "radar", "glitch", "bubbles", "coins", "basketballs"},
		"mic":   {"mic", "megaphone", "studio", "podcast", "karaoke", "headset", "walkie", "tincan", "radio", "reporter", "clawdspark", "ribbon", "elvis", "carbon", "condenser", "banana", "tentacle", "cthulhu", "coinmic", "holomic", "ringmic", "spacecomm", "bulbmic", "pumpkinmic"},
		"button": {"nuclear", "mushroom", "glassbox", "keyswitch", "easy", "slots", "disco", "confetti", "gong", "bikehorn", "spark", "buzzer", "goalhorn", "selfdestruct", "hologram", "warp"},
		"floor": floors,
		"paint": paints,
		// The lounge's carpets, or any of the office's floors
		"lounge":      append([]string{"navy", "teal", "plum", "sand"}, floors...),
		"loungePaint": append([]string{"auto"}, paints...),
		"corner": {"arcade", "vending", "foosball", "fishtank", "books", "dragon", "pinball", "jukebox", "pool", "piano", "drums",
			"telescope", "jellyfish", "bonsai", "jungle", "coffee", "servers", "printer", "boxing", "treadmill", "hottub", "ballpit",
			"campfire", "bubbler", "neonbox", "wallbox", "djtable", "turntable", "boombox", "hifi", "synth", "cauldron", "graveyard",
			"lanterns", "pumpkinrack", "halfcourt", "arcadehoop", "ballrack", "studiodesk", "bench", "coffinbox", "skeletonmodel", "xmastree", "fireplace", "sleigh", "carolbox", "woodstove", "cocoabar", "heartbox", "candledinner", "plumtree", "lionhead", "watchparty", "noodlebar", "periscope", "sonar", "espressobar", "saloonbar", "pianola", "discobooth", "stadiumorgan", "valveradio", "guzheng", "holobooth", "gramophone", "cosmicsynth", "koipond", "singingbowls", "guitarhook", "clawmachine", "trainset", "easel", "orrery", "copier", "teslacoil", "chesstable", "butterflyhouse", "switchboard", "puttinggreen", "retrofridge", "readingchair", "potterywheel", "projector", "birdfeeder", "vivarium", "claudecade", "sparkfloorlamp", "eldritchaltar", "deepaquarium", "miskatonicnook", "coldsafe", "noderack", "bullstatue", "coinfountain", "sleepybear", "leakbucket", "miningrig", "parcstand", "celticsmascot", "bullsmascot", "heatmascot", "spursmascot", "rocketsmascot", "raptorsmascot", "sunsmascot", "nuggetsmascot", "thundermascot", "grizzliesmascot", "hawksmascot", "hornetsmascot", "pacersmascot", "bucksmascot", "wolvesmascot", "jazzmascot", "kingsmascot", "magicmascot", "pistonsmascot", "pelicansmascot", "cavsmascot", "mavsmascot", "sixersmascot", "blazersmascot", "wizardsmascot", "clippersmascot", "netsmascot", "lakersmascot", "knicksmascot", "warriorsmascot", "marvel1worthy", "marvel1webcorner", "marvel2throne", "marvel2herbgarden", "ironmanhulkbuster", "ironmangantry", "batcavebatmobile", "batcavepenny", "dcmixphonebooth", "dcmixtreadmill", "pokemonhealer", "pokemonsnorlax", "lolnexus", "lolbaron", "wowforge", "wowgryphon", "gtapayspray", "gtaphonebooth", "destinycryptarch", "destinysparrowbay", "starwarsholotable", "starwarscantina", "girly1convertible", "girly1wardrobe", "girly2dessert", "girly2plushpile", "narcosmoney", "narcoslab", "apple1geniusbar", "apple1trees", "apple2bigmac", "apple2garage", "jewishsukkah", "jewishstudy", "muslimprayer", "muslimfountain", "christianpew", "christianorgan", "buddhistaltar", "buddhistmeditation", "volcanolavapit", "volcanothrone", "kitchenline", "kitchenpass", "hospitalbed", "hospitalcrash", "noirinterrogation", "noircell", "madlabslab", "madlabtank", "casinoslots", "casinoroulette", "orientdining", "orientloco", "halloweendxcauldron", "halloweendxtree", "catcafetree", "catcafecounter", "spiderversemural", "spiderversegohome", "xmencerebro", "xmensentinelhead", "f1simrig", "f1garage", "ufcheavybag", "ufcoctagon", "nyloftrecords", "nyloftplants", "pariscafeterrace", "pariscafepastry", "muertosofrenda", "muertoscart", "submarinedeepairlock", "submarinedeeptank", "arcticbasewoodstove", "arcticbasesnowmobile", "sakuratree", "sakurastall", "potterplatform", "potterfireplace", "lotrprancing", "lotrdoom", "breakingbadrvcorner", "breakingbadreactor", "strangergate", "strangercastle", "gotironthrone", "gotmaptable", "simpsonstv", "simpsonskwikemart", "bondbaccarat", "bondlaser", "duneworm", "duneconservatory", "onepiecefigurehead", "onepiecegalley", "eldenringgracesite", "eldenringminortree", "sevmde", "sevwellnessroom", "offreception", "offkitchen", "peakybar", "peakyfireplace", "madmbarcart", "madmhifi", "squidredlight", "squidglassbridge", "boysstage", "boysstatue", "falloutvaultcog", "falloutpowerarmor", "rickship", "rickportallab", "mirrorpod", "mirrorserver", "westpianocorner", "westrepairbay", "bttfdelorean", "bttfdinerbooth", "jptrex", "jphatchery", "wickbar", "wickvault", "kingsmanfitting", "kingsmanroundtable", "wonkariver", "wonkagarden", "weslobby", "wesbakery", "mibarrivals", "mibbigscreen", "tronarcade", "troncyclearena", "dbzgravity", "dbzkamehouse", "amongusreactor", "amonguscafeteria", "tokyoyatai", "tokyoshrine", "egyptsarc", "egyptbrazier", "vikinghearth", "vikinghighseat", "aztecshrine", "aztecserpenthead", "obstelescope", "obsplanetarium", "tikibar", "tikifirepit", "dinerjukebox", "dinercounter", "courtbench", "courtjurybox", "arteasel", "artgallery", "aquariumtunnel", "aquariumtouchpool", "circuscannon", "circuscalliope", "museumrex", "museumgem", "northpoletree", "northpolesleigh", "soundstagedolly", "soundstageset", "cultshrine", "cultgathering", "esportsstage", "esportsstream", "cavemanfire", "cavemanmount", "lodgealtar", "lodgethrone", "prepperstation", "prepperstockpile", "paradisevault", "paradisesurveil", "startrektransporter", "startrekwarpcore", "gentlemenlounge", "gentlemenbilliards", "groklaunch", "grokbotrack", "none"},
		"sky":       {"live", "day"},
		"season":    {"calendar", "off"},
		"sound":     {"off", "quiet", "normal"},
		"ambience":  {"on", "off"},
		"soundWhen": {"always", "front"},
		"desk":      {"wood", "walnut", "white", "black"},
		"workspace": workspaces,
		"monitor":   {"standard", "crt", "holo", "brass", "crystal", "gilt", "ice", "ultrawide", "dual", "laptop", "bamboo", "tablet", "portrait", "broadcast", "ghostcrt", "ouija", "tombstone", "candycane", "giftwrap", "knitmon", "heartframe", "locket", "lanternmon", "scoreboard", "catnap", "mossframe", "satdish", "bookstack", "woodcase", "nestmonitor", "claudeterm", "sparksaver", "wirelessset", "scryingscreen", "orderbook", "greencandles", "redcandles", "stickerback", "matchcast", "lakerscast", "celticscast", "warriorscast", "knickscast", "bullscast", "heatcast", "spurscast", "thundercast", "wolvescast", "nuggetscast", "hawkscast", "netscast", "hornetscast", "cavscast", "mavscast", "pistonscast", "rocketscast", "pacerscast", "clipperscast", "grizzliescast", "buckscast", "pelicanscast", "magiccast", "sixerscast", "sunscast", "blazerscast", "kingscast", "raptorscast", "jazzcast", "wizardscast", "marvel1shieldmon", "marvel2kimoyo", "ironmanjarvis", "batcavescreen", "dcmixlanternmon", "pokemondex", "lolhexmon", "wowportal", "gtawanted", "destinytraveler", "starwarsmon", "girly1mon", "girly2mon", "narcosmon", "apple1imac", "apple2classic", "jewishmon", "muslimmon", "christianmon", "buddhistmon", "volcanomon", "kitchenmon", "hospitalmon", "noirmon", "madlabmon", "casinomon", "orientmon", "halloweendxmon", "catcafemon", "spiderversemon", "xmenmon", "f1mon", "ufcmon", "nyloftmon", "pariscafemon", "muertosmon", "submarinedeepmon", "arcticbasemon", "sakuramon", "pottermon", "lotrmon", "breakingbadmon", "strangermon", "gotmon", "simpsonsmon", "bondmon", "dunemon", "onepiecemon", "eldenringmon", "sevmon", "offmon", "peakymon", "madmmon", "squidmon", "boysmon", "falloutmon", "rickmon", "mirrormon", "westmon", "bttftimecircuits", "jpmon", "wickmon", "kingsmanmon", "wonkamon", "wesmon", "mibmon", "tronmon", "dbzmon", "amongusmon", "tokyomon", "egyptmon", "vikingmon", "aztecmon", "obsmon", "tikimon", "dinermon", "courtmon", "artmon", "aquariummon", "circusmon", "museummon", "northpolemon", "soundstagemon", "cultmon", "esportsmon", "cavemanmon", "lodgemon", "preppermon", "paradiseterminal", "startrekconsole", "gentlemenwoodcase", "grokmon", "grokbotmon"},
		"door":      {"standard", "haunted", "wreathdoor", "mailboxdoor", "lunardoor", "lockerdoor", "saloondoor", "glassdoor", "dutchdoor", "screendoor", "vaultdoor", "marvel1door", "marvel2sanctumdoor", "ironmandoor", "batcaveclock", "dcmixhalldoor", "pokemondoor", "loldoor", "wowdoor", "gtadoor", "destinydoor", "starwarsdoor", "girly1door", "girly2door", "narcosdoor", "apple1door", "apple2door", "jewishdoor", "muslimdoor", "christiandoor", "buddhistdoor", "volcanodoor", "kitchendoor", "hospitaldoor", "noirdoor", "madlabdoor", "casinodoor", "orientdoor", "halloweendxdoor", "catcafedoor", "spiderversedoor", "xmendoor", "f1door", "ufcdoor", "nyloftdoor", "pariscafedoor", "muertosdoor", "submarinedeepdoor", "arcticbasedoor", "sakuradoor", "potterdoor", "lotrdoor", "breakingbaddoor", "strangerdoor", "gotdoor", "simpsonsdoor", "bonddoor", "dunedoor", "onepiecedoor", "eldenringfog", "sevdoor", "offdoor", "peakydoor", "madmdoor", "squiddoor", "boysdoor", "falloutdoor", "rickdoor", "mirrordoor", "westdoor", "bttfdoor", "jpgatedoor", "wickdoor", "kingsmanfittingdoor", "wonkadoor", "wesdoor", "mibdoor", "trondoor", "dbzdoor", "amongusdoor", "tokyodoor", "egyptdoor", "vikingdoor", "aztecdoor", "obsdoor", "tikidoor", "dinerdoor", "courtdoor", "artdoor", "aquariumdoor", "circusflap", "museumarch", "northpolewreathdoor", "soundstagedoor", "cultdoor", "esportsdoor", "cavemandoor", "lodgedoor", "prepperhatch", "paradiseblastdoor", "startrekturbolift", "gentlemenbookdoor", "grokdoor"},
		"drift":     {"none", "snow", "hearts", "lunar", "confetti", "seamist", "greenburst", "marvel1webzip", "marvel2snap", "ironmanflyby", "batcaveswarm", "dcmixspeedster", "pokemonthrow", "lolpings", "wowlevelup", "gtasiren", "destinyengrams", "starwarslightspeed", "girly1glitter", "girly2bubbles", "narcosmoneyrain", "apple1icons", "apple2toasters", "jewishlights", "muslimlanterns", "christianstar", "buddhistleaves", "volcanoembers", "kitchensteam", "hospitalsiren", "noirrain", "madlabbubbles", "casinocardrain", "orientsteam", "halloweendxghost", "catcafeyarnball", "spiderverseswing", "xmenbamf", "f1flyby", "ufcflashes", "nylofttrain", "pariscafenotes", "muertosdrift", "submarinedeepbubbles", "arcticbaseaurora", "sakuraflurry", "potterletters", "lotrfirework", "breakingbadcash", "strangerspores", "gotdragonshadow", "simpsonsdonuts", "bonddots", "dunespicedrift", "onepiecebubbles", "eldenringleaves", "sevballoons", "offplanes", "peakysmoke", "madmsmoke", "squidmoney", "boysstreamers", "falloutdust", "rickspark", "mirrorglitch", "westdust", "bttfsparks", "jptremor", "wickcoinrain", "kingsmanbrolly", "wonkabubbles", "wespostcards", "mibflash", "tronderez", "dbzki", "amonguseject", "tokyorain", "egyptsand", "vikingembers", "aztecleaves", "obsstars", "tikipetals", "dinernotes", "courtpapers", "artsplatter", "aquariumbubbles", "circusrelease", "museumdust", "northpolesleighby", "soundstagebeams", "cultpetals", "esportspyro", "cavemanembers", "lodgeincense", "prepperalert", "paradisedust", "startrekalertdrift", "gentlemensmoke"},
		"fireworks": {"off", "midnight", "nightly"},
		"chair":     {"teal", "black", "red", "grey", "green"},
		"staff":     {"some", "none", "many"},
		"arrivals":  {"animated", "plain"},
		"calm":      {"off", "on"},
		"preset":    roomStyles,
	}
	wallChoices = []string{"window", "whiteboard", "pinboard", "poster", "shelf", "plant", "bare",
		"viewport", "porthole", "panel", "pipes", "seaPorthole", "map", "flag", "cutlasses", "tapestry", "portrait", "stained", "shield",
		"frost", "skis", "weather", "torch", "arch", "neon", "tv", "hex", "pegboard", "roller", "bike", "mirror", "timer", "climbing",
		"vines", "tiki", "blind", "cabin", "trail", "paddles", "pumpkins", "lanternhang", "cobweb", "ghosts", "broom", "leafgarland",
		"hauntedhoop", "spookyboard", "candycorn", "hoop", "scorebug", "lowerthird", "onair", "schedule", "pennants", "retired",
		"playboard", "garland", "scarfhooks", "heartgarland", "bemine", "lunarlanterns", "pennantline", "scarfwall", "rainwindow", "neonsign", "fishporthole", "chalkmenu", "pendantlamps", "wanted", "countdown", "streamers", "balloons", "earthwindow", "paperscreen", "leafwindow", "ropeladder", "cuckoo", "photostring", "dartboard", "splitflap", "periodictable", "growshelf", "mugrack", "wallfan", "kite", "windchime", "claudeneon", "modelposter", "hellocard", "contextclock", "tentacleporthole", "starchartodd", "followingportrait", "sunkencity", "eldertapestry", "tickerboard", "candlechart", "marketclocks", "moonposter", "upchart", "moonwindow", "redchart", "stormpane", "hanginthere", "blackout", "denposters", "floodlights", "framedjersey", "lakersbanner", "celticsbanner", "warriorsbanner", "knicksbanner", "bullsbanner", "heatbanner", "spursbanner", "thunderbanner", "wolvesbanner", "nuggetsbanner", "hawksbanner", "netsbanner", "hornetsbanner", "cavsbanner", "mavsbanner", "pistonsbanner", "rocketsbanner", "pacersbanner", "clippersbanner", "grizzliesbanner", "bucksbanner", "pelicansbanner", "magicbanner", "sixersbanner", "sunsbanner", "blazersbanner", "kingsbanner", "raptorsbanner", "jazzbanner", "wizardsbanner", "marvel1shieldwall", "marvel1hammerwall", "marvel1spidewindow", "marvel1assemble", "marvel1hulkhole", "marvel1bugle", "marvel2gauntlet", "marvel2sanctumwindow", "marvel2portal", "marvel2awesomemix", "marvel2tapestry", "marvel2lokihelm", "ironmanarmory", "ironmanblueprint", "ironmanmark1", "ironmanexpo", "ironmanheart", "ironmanpegboard", "batcavesignal", "batcavesuits", "batcavestalactites", "batcaveboard", "batcavecard", "batcavebatarangs", "dcmixplanetglobe", "dcmixshield", "dcmixlasso", "dcmixflashwindow", "dcmixtrident", "dcmixharley", "pokemonbadges", "pokemonmap", "pokemonwindow", "pokemonposter", "pokemonballshelf", "pokemonsign", "lolminimap", "lolbanner", "lolcrest", "lolwindow", "lolabilities", "lolpenta", "wowlion", "wowhordebanner", "wowquestboard", "wowwindow", "wowshield", "wowmap", "gtasunset", "gtamap", "gtaneon", "gtaposter", "gtagrove", "gtaplates", "destinywindow", "destinyhunter", "destinytitan", "destinywarlock", "destinydirector", "destinyxur", "starwarshyper", "starwarssuns", "starwarssabers", "starwarscarbonite", "starwarsbanner", "starwarsplans", "girly1mirror", "girly1neon", "girly1heels", "girly1carposter", "girly1heartclock", "girly1curtains", "girly2rainbow", "girly2kittyframe", "girly2polaroids", "girly2moon", "girly2plushshelf", "girly2cloudclock", "narcoswanted", "narcosmap", "narcosportrait", "narcospalms", "narcossafe", "narcosjersey", "apple1logo", "apple1window", "apple1poster", "apple1phones", "apple1watches", "apple1hello", "apple2logo", "apple2think", "apple21984", "apple2floppy", "apple2macpaint", "apple2bomb", "jewisharch", "jewishbooks", "jewishpapercut", "jewishhanukkiah", "jewishshabbat", "jewishshofar", "muslimmashrabiya", "muslimlantern", "muslimtilepanel", "muslimarabesque", "muslimarch", "muslimquranshelf", "christianstainedglass", "christiancross", "christianrosewindow", "christianbibleshelf", "christiansconce", "christianwreath", "buddhistflags", "buddhistaltarshelf", "buddhistlotusscroll", "buddhistmoonwindow", "buddhistwheel", "buddhistlanterns", "volcanocaldera", "volcanomap", "volcanobeacon", "volcanopipes", "volcanoporthole", "volcanocountdown", "kitchenrail", "kitchenpans", "kitchenknives", "kitchenheatlamps", "kitchenhood", "kitchenspices", "hospitalecg", "hospitalxray", "hospitaleyechart", "hospitalclock", "hospitalexit", "hospitalgloves", "noirblinds", "noirevidence", "noirheight", "noircitymap", "noirhatrack", "noirfrosted", "madlablever", "madlabladder", "madlabjars", "madlabchalk", "madlabgauges", "madlabwindow", "casinomarquee", "casinowheel", "casinoneonsign", "casinoflush", "casinojackpot", "casinochandelier", "orientclock", "orientwindow", "orientposter", "orientluggage", "orientsconce", "orientroute", "halloweendxmoon", "halloweendxportrait", "halloweendxclock", "halloweendxgarland", "halloweendxshelf", "halloweendxmirror", "catcafeshelves", "catcafeportraits", "catcafewindow", "catcafebunting", "catcafeyarn", "catcafeplanter", "spiderversewindow", "spiderversecomic", "spiderverseportal", "spiderversemasks", "spiderversespot", "spiderversepennant", "xmenlogo", "xmenblackbird", "xmenroster", "xmenschool", "xmenalert", "xmenhelmet", "f1lights", "f1tyres", "f1helmetcase", "f1timing", "f1circuit", "f1trophies", "ufcfence", "ufcposter", "ufcbelt", "ufcclock", "ufcgloves", "ufcspeedbag", "nyloftwindow", "nyloftsubway", "nyloftshelf", "nyloftneon", "nyloftclock", "nyloftbike", "pariscafewindow", "pariscafemenu", "pariscafemirror", "pariscafeposter", "pariscafeclock", "pariscafebottles", "muertospapel", "muertosretablo", "muertoscatrina", "muertostinsun", "muertosguitar", "muertoswindow", "submarinedeepviewport", "submarinedeepdepth", "submarinedeepmap", "submarinedeepvalves", "submarinedeeplocker", "submarinedeepsamples", "arcticbasewindow", "arcticbasemap", "arcticbaseweather", "arcticbaseradio", "arcticbaseskis", "arcticbasechart", "sakurabranch", "sakurawindow", "sakurafan", "sakurastreamers", "sakuraprint", "sakuracalendar", "potterportrait", "potterhousepoints", "potterwindow", "potterbroom", "pottermap", "potterbanners", "lotrwindow", "lotrmap", "lotreye", "lotrsting", "lotrgondor", "lotrrohan", "breakingbadlogo", "breakingbadpollos", "breakingbadheisenberg", "breakingbadsaul", "breakingbadwindow", "breakingbadmasks", "strangerlights", "strangerhellfire", "strangerwindow", "strangerscoops", "strangerclock", "strangerhawkins", "gotstark", "gotlannister", "gottargaryen", "gotwindow", "gotswords", "gotweirwood", "simpsonsboat", "simpsonslardlad", "simpsonskrusty", "simpsonswindow", "simpsonsaccident", "simpsonsduff", "bondgunbarrel", "bond007", "bondworldmap", "bondwindow", "bondgadgets", "bondparachute", "dunewindow", "duneatreidesbanner", "duneharkonnen", "dunebullhead", "dunecrysknife", "dunespicemap", "onepiecejollyroger", "onepiecewanted", "onepieceporthole", "onepiecehelm", "onepiecemap", "onepieceswords", "eldenringerdtreewindow", "eldenringsigil", "eldenringmap", "eldenringmoonveil", "eldenringranni", "eldenringdied", "sevkierportrait", "sevprinciples", "sevclock", "sevbreakroom", "sevbanner", "sevlight", "offwindow", "offmotivation", "offwhiteboard", "offcorkboard", "offclock", "offdundieplaque", "peakyoddsboard", "peakywindow", "peakyportrait", "peakywhiskyshelf", "peakycap", "peakyclock", "madmwindow", "madmart", "madmclock", "madmboard", "madmshelf", "madmposter", "squidshapes", "squidpiggybank", "squidladders", "squidnumbers", "squidguardframe", "squidclock", "boyswindow", "boysseven", "boysstat", "boyslogo", "boysaward", "boysclock", "falloutvaultdoor", "falloutvaultboy", "falloutposter", "falloutterminal", "falloutsign", "falloutclock", "rickportalwall", "rickpegboard", "rickchalkboard", "rickjars", "rickflag", "rickclock", "mirrorscreen", "mirrorrating", "mirrorwindow", "mirrorblocked", "mirrorart", "mirrorclock", "westhostvat", "westvitruvian", "westwindow", "westmaze", "westwanted", "westantlers", "bttfclocktower", "bttfneondiner", "bttfroadsign", "bttfsaveclock", "bttfmenuboard", "bttfwallclock", "jpgate", "jpbanner", "jpwarnfence", "jpparkmap", "jpfossilwall", "jppaddocksign", "wickneon", "wickportrait", "wickclock", "wickmirror", "wickboard", "wicksconce", "kingsmantriplemirror", "kingsmansuitrack", "kingsmanfabrics", "kingsmanportrait", "kingsmanclock", "kingsmanshears", "wonkagates", "wonkapipes", "wonkawallpaper", "wonkamarquee", "wonkacandyshelf", "wonkaclock", "wesportrait", "wesclock", "wessunburst", "wespigeonholes", "weslandscape", "wesbanner", "mibworldmap", "mibscreens", "mibzoneclocks", "mibposter", "mibtubes", "mibwanted", "trongrid", "tronrecognizer", "trontower", "tronmap", "tronportal", "tronleaderboard", "dbzturtlekanji", "dbzbanner", "dbzcapsulesign", "dbzscroll", "dbzpunchboard", "dbzclock", "amongusvent", "amongusmap", "amonguswiring", "amongusviewport", "amongustasklist", "amonguswarning", "tokyosign", "tokyolanterns", "tokyonoren", "tokyobillboard", "tokyovending", "tokyocables", "egypthiero", "egyptanubis", "egyptpharaoh", "egypttorch", "egyptankh", "egyptfalcon", "vikingshield", "vikingantlers", "vikingtapestry", "vikinghornrack", "vikingrunestone", "vikingtorch", "aztecsunstone", "aztecidol", "aztecserpent", "aztecmask", "aztecvines", "aztecglyphs", "obschart", "obsconstellation", "obsmoonphase", "obszodiac", "obsplanetposter", "obssextant", "tikimask", "tikisurfboard", "tikitorch", "tikipuffer", "tikisign", "tikihula", "dinerneon", "dinerclock", "dinermenu", "dinerpie", "dinershake", "dinerplates", "courtscales", "courtflag", "courtseal", "courtclock", "courtbooks", "courtportrait", "artcanvas", "artpalette", "artsketch", "artskylight", "artshelf", "artposter", "aquariumreef", "aquariumjelly", "aquariumshark", "aquariumseahorse", "aquariumstar", "aquariumtube", "circusposter", "circusbunting", "circussign", "circustrapeze", "circuswheel", "circusmirror", "museumtrex", "museumdiorama", "museumbutterflies", "museumgems", "museumportrait", "museummap", "northpolewreath", "northpolestockings", "northpolewindow", "northpolelist", "northpolelights", "northpolegears", "soundstagerig", "soundstageposter", "soundstagescreen", "soundstagestar", "soundstagestoryboard", "soundstagereel", "culteye", "cultsun", "cultportrait", "culttenets", "cultwindow", "cultcandles", "esportsscreen", "esportsbracket", "esportsleaderboard", "esportsbanner", "esportsneon", "esportsshelf", "cavemanhandprints", "cavemanhunt", "cavemanbones", "cavemanwheel", "cavemanwindow", "cavemantorch", "lodgeeye", "lodgepillars", "lodgeportrait", "lodgecharter", "lodgesymbols", "lodgesconce", "prepperboard", "preppershelf", "prepperhazard", "preppertools", "prepperclock", "preppervent", "paradisesealplaque", "paradisefakewindow", "paradisemonitors", "paradiseblasthatch", "paradisedoomsday", "paradiseportrait", "startrekviewscreen", "startreklcars", "startrekplaque", "startrekalert", "startrekstarmap", "startrekport", "gentlemenstag", "gentlemenportrait", "gentlemenguncabinet", "gentlemenmap", "gentlemengrowroom", "gentlemenhunt", "grokneon", "grokgalaxy", "grok42", "grokboard", "grokwindow", "grokfun", "grokbotmissions", "grokbotroutine", "grokbotmailbox", "grokbotstatus"}
	standardWall = []string{"window", "whiteboard", "window", "pinboard", "window"}
	// The pets an office can have.
	pets = []string{"cat", "dog", "lizard", "monkey", "llama", "penguin", "parrot", "owl", "capybara", "redpanda", "corgi", "hamster",
		"robodog", "bat", "snail", "fox", "axolotl", "blackcat", "goat", "raven", "ghost", "spider", "reindeer", "snowman", "snowhare", "cupid", "lovebirds", "drone", "tumbleweed", "hedgehog", "tortoise", "treefrog", "marsrover", "ferret", "duckling", "tinrobot", "guineapig", "robovac", "hermitcrab", "hen", "sparkling", "shoggoth", "minicthulhu", "rgbcat", "marvel1jeff", "marvel1throg", "marvel2groot", "marvel2goose", "ironmandummy", "ironmanlegion", "batcaveace", "batcavecow", "dcmixkrypto", "dcmixstreaky", "pokemonpikachu", "pokemoncharmander", "lolporo", "lolminion", "wowmurloc", "wowwhelp", "gtachop", "gtapigeon", "destinyghost", "destinyservitor", "starwarsr2", "starwarsgrogu", "girly1poodle", "girly1flamingo", "girly2kitty", "girly2bunny", "narcoshippo", "narcosparrot", "apple1siri", "apple1ipod", "apple2mac", "apple2dogcow", "jewishdove", "jewishhoopoe", "muslimcat", "muslimkitten", "christianlamb", "christianrobin", "buddhistdeer", "buddhistkoi", "volcanosalamander", "volcanocat", "volcanogolem", "kitchencat", "kitchenlobster", "kitchenmouse", "hospitaldog", "hospitalbot", "hospitalbunny", "noircat", "noirpigeon", "noirhound", "madlabrat", "madlabslime", "madlabbrainbot", "casinopug", "casinotiger", "casinodove", "orientscottie", "orientsiamese", "orientpeke", "halloweendxwisp", "halloweendxpumpkin", "halloweendxbatcat", "halloweendxghostdog", "catcafeginger", "catcafecalico", "catcafeblue", "catcafetabby", "catcafepersian", "catcafetortie", "catcafesphynx", "catcafekitten", "catcafemaine", "catcafebengal", "catcafepoodle", "spiderverseham", "spiderversespider", "spiderversespdr", "xmenlockheed", "xmensentinel", "f1roscoe", "f1drone", "ufcgymcat", "ufcpug", "nyloftcat", "nyloftpigeon", "nyloftdachshund", "pariscafepoodle", "pariscafesparrow", "pariscafecat", "muertosxolo", "muertosalebrije", "muertosmonarch", "submarinedeepoctopus", "submarinedeepjelly", "submarinedeepangler", "arcticbasehusky", "arcticbasefox", "arcticbasepuffin", "sakurashiba", "sakuramejiro", "sakurabunny", "potterhedwig", "pottercrookshanks", "potterfawkes", "lotrgollum", "lotreagle", "lotrbill", "breakingbadfly", "breakingbadtortoise", "breakingbadroadrunner", "strangerdemodog", "strangermews", "strangerdemobat", "gotdragon", "gotdirewolf", "gotraven", "simpsonsdog", "simpsonssnowball", "simpsonsplopper", "bondpersian", "bondrc", "bondgyro", "dunemouse", "dunesandworm", "duneornithopter", "onepiecechopper", "onepiecesnail", "onepiecegull", "eldenringtorrent", "eldenringjellyfish", "eldenringpastor", "sevgoat", "sevkier", "offcat", "offbat", "peakyhorse", "peakyterrier", "madmcat", "madmdachshund", "squiddoll", "squidpiggy", "boysbulldog", "boysoctopus", "falloutdog", "falloutmrhandy", "rickmeeseeks", "ricksnowball", "mirrordog", "mirrorbee", "westhorse", "westfly", "bttfeinstein", "bttfdinermouse", "jpraptor", "jpcompy", "wickdog", "wickcat", "kingsmanpug", "kingsmanpigeon", "wonkasquirrel", "wonkagummy", "wescat", "wesfox", "mibpug", "mibroach", "tronbit", "tronpup", "dbzdragon", "dbzturtle", "amonguspet", "amongusufo", "tokyocat", "tokyotanuki", "egyptcat", "egyptscarab", "vikingraven", "vikinggoat", "aztecjaguar", "aztecmacaw", "obsowl", "obscat", "tikiparrot", "tikicrab", "dinercat", "dinerpup", "courtowl", "courtcat", "artcat", "artbird", "aquariumpenguin", "aquariumotter", "circusmonkey", "circusseal", "circuspony", "museumdodo", "museumtortoise", "museummoth", "northpolereindeer", "northpolehusky", "northpolerobin", "soundstagedog", "soundstagecat", "soundstagepigeon", "cultdove", "cultgoose", "cultcat", "esportsmascot", "esportsdrone", "esportscat", "cavemandino", "cavemanmammoth", "cavemanptero", "lodgeraven", "lodgeowl", "lodgecat", "preppercanary", "prepperrat", "prepperdog", "paradisedog", "paradisedrone", "startrektribble", "startrekcat", "gentlemenhound", "gentlemenpheasant", "grokdrone"}
)

// The most pets an office has at once.
const petsMost = 3

// UnmarshalJSON keeps what it knows of the room it's given.
func (r *Room) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)
	text := func(data json.RawMessage) string {
		var s string
		json.Unmarshal(data, &s)
		return s
	}
	var wall []json.RawMessage
	json.Unmarshal(raw["wall"], &wall)
	*r = Room{Mic: text(raw["mic"]), Wave: text(raw["wave"]), Button: text(raw["button"]), Floor: text(raw["floor"]), Paint: text(raw["paint"]), Lounge: text(raw["lounge"]), LoungePaint: text(raw["loungePaint"]),
		Corner: text(raw["corner"]), Sky: text(raw["sky"]), Season: text(raw["season"]), Sound: text(raw["sound"]), Ambience: text(raw["ambience"]), SoundWhen: text(raw["soundWhen"]),
		Desk: text(raw["desk"]), Workspace: text(raw["workspace"]), Monitor: text(raw["monitor"]), Door: text(raw["door"]), Drift: text(raw["drift"]), Fireworks: text(raw["fireworks"]), Chair: text(raw["chair"]),
		Staff: text(raw["staff"]), Arrivals: text(raw["arrivals"]), Calm: text(raw["calm"]),
		Preset: text(raw["preset"])}
	for _, place := range wall {
		r.Wall = append(r.Wall, text(place))
	}
	// The pets as a list, or one by its id, as an office had before it could have more
	var pets []json.RawMessage
	if json.Unmarshal(raw["pet"], &pets) != nil {
		pets = []json.RawMessage{raw["pet"]}
	}
	for _, pet := range pets {
		r.Pet = append(r.Pet, text(pet))
	}
	var team map[string]json.RawMessage
	json.Unmarshal(raw["team"], &team)
	r.Team = map[string]TeamMember{}
	for project, given := range team {
		var parts map[string]json.RawMessage
		json.Unmarshal(given, &parts)
		r.Team[project] = TeamMember{Skin: text(parts["skin"]), Hair: text(parts["hair"]), Hairstyle: text(parts["hairstyle"]),
			Extra: text(parts["extra"]), Outfit: text(parts["outfit"]), Colour: text(parts["colour"]), Desk: text(parts["desk"]),
			Workspace: text(parts["workspace"]), Name: text(parts["name"])}
	}
	json.Unmarshal(raw["dress"], &r.Dress)
	json.Unmarshal(raw["plan"], &r.Plan)
	*r = r.checked()
	return nil
}

// MarshalJSON writes the room as checked, so a layout kept before rooms had
// a furnishing still sends the standard one.
func (r Room) MarshalJSON() ([]byte, error) {
	type plain Room
	return json.Marshal(plain(r.checked()))
}

// checked is the room with anything unknown as standard.
func (r Room) checked() Room {
	choose := func(part, given string) string {
		if slices.Contains(roomChoices[part], given) {
			return given
		}
		return roomChoices[part][0]
	}
	checked := Room{Mic: choose("mic", r.Mic), Wave: choose("wave", r.Wave), Button: choose("button", r.Button), Floor: choose("floor", r.Floor), Paint: choose("paint", r.Paint),
		Lounge: choose("lounge", r.Lounge), LoungePaint: choose("loungePaint", r.LoungePaint), Corner: choose("corner", r.Corner), Sky: choose("sky", r.Sky),
		Pet: checkedPets(r.Pet), Season: choose("season", r.Season), Sound: choose("sound", r.Sound), Ambience: choose("ambience", r.Ambience), SoundWhen: choose("soundWhen", r.SoundWhen),
		Desk: choose("desk", r.Desk), Workspace: choose("workspace", r.Workspace), Monitor: choose("monitor", r.Monitor),
		Door: choose("door", r.Door), Drift: choose("drift", r.Drift), Fireworks: choose("fireworks", r.Fireworks), Chair: choose("chair", r.Chair),
		Staff: choose("staff", r.Staff), Arrivals: choose("arrivals", r.Arrivals), Calm: choose("calm", r.Calm), Preset: choose("preset", r.Preset), Wall: slices.Clone(standardWall), Dress: r.Dress.checked(),
		Plan: r.Plan.checked()}
	for i := range checked.Wall {
		if i < len(r.Wall) && slices.Contains(wallChoices, r.Wall[i]) {
			checked.Wall[i] = r.Wall[i]
		}
	}
	// The first projects by name, as many as a team remembers.
	checked.Team = map[string]TeamMember{}
	projects := make([]string, 0, len(r.Team))
	for project := range r.Team {
		if project != "" {
			projects = append(projects, project)
		}
	}
	slices.Sort(projects)
	for _, project := range projects[:min(len(projects), teamLimit)] {
		checked.Team[project] = r.Team[project].checked()
	}
	return checked
}

// checkedPets is the pets given that an office can have, each once and as
// many as it has at once, in their order: none is an empty list, not null.
func checkedPets(given []string) []string {
	checked := []string{}
	for _, pet := range given {
		if slices.Contains(pets, pet) && !slices.Contains(checked, pet) && len(checked) < petsMost {
			checked = append(checked, pet)
		}
	}
	return checked
}

// ButtonRow is a row of buttons, tall or short, sharing its width by weight.
type ButtonRow struct {
	Tall    bool     `json:"tall"`
	Buttons []Button `json:"buttons"`
}

// Button is one of the deck's own, named by what it does, or one made in
// setup, which types a command or presses a key. The phone knows a made
// button only by its id: what it types stays here.
type Button struct {
	ID    string `json:"id"`
	Width int    `json:"width"`
	Label string `json:"label,omitempty"`
	Text  string `json:"text,omitempty"`
	Key   string `json:"key,omitempty"`
	// Refused while the agent is busy, as /clear is.
	WhenFree bool `json:"whenFree,omitempty"`
	// Asks for a second tap, as /clear does.
	Confirm bool `json:"confirm,omitempty"`
	// The agents it is for; none means every agent.
	Agents []string `json:"agents,omitempty"`
}

const (
	maxRows      = 4
	maxPerRow    = 4
	maxWidth     = 3
	labelLimit   = 16
	commandLimit = 200
)

var (
	// The deck's own buttons. Max and the steps set Claude Code's effort, as
	// the dial does.
	builtInButtons = []string{"accept", "reject", "mic", "esc", "send", "clear", "up", "down", "max", "efforts"}
	// The keys a made button can press.
	buttonKeys = map[string]Keystroke{
		"enter": press(Enter), "escape": press(Escape), "tab": press(TabKey), "shift-tab": press(ShiftTab),
		"up": press(Up), "down": press(Down), "ctrl-c": press(CtrlC),
	}
	madeButtonID = regexp.MustCompile(`^custom-[a-z0-9]{1,16}$`)
	controlCode  = regexp.MustCompile(`[\x00-\x1f\x7f]`)
)

// standardLayout is today's deck: a key per session, the dial and stick,
// then ✓ ✕ and the mic over esc, send and /clear.
func standardLayout() Layout {
	row := func(tall bool, ids ...string) ButtonRow {
		buttons := make([]Button, len(ids))
		for i, id := range ids {
			buttons[i] = Button{ID: id, Width: 1}
		}
		return ButtonRow{Tall: tall, Buttons: buttons}
	}
	return Layout{Keys: Auto, Room: Room{}.checked(), Dial: true, Stick: true, Rows: []ButtonRow{
		row(true, "accept", "reject", "mic"), row(false, "esc", "send", "clear"),
	}}
}

// parseLayout reads a layout sent by the setup page or found in a profile's
// file. What it leaves out is the default: Auto keys, no office, a standard
// room, the dial and the stick showing, no buttons.
func parseLayout(data []byte) (Layout, error) {
	layout := Layout{Keys: Auto, Room: Room{}.checked(), Dial: true, Stick: true}
	if err := json.Unmarshal(data, &layout); err != nil {
		return Layout{}, errors.New("That isn't a layout this deck can use.")
	}
	return layout.checked(), nil
}

// checked keeps what the deck can draw and press: known buttons, each of
// the deck's own once, within the limits.
func (l Layout) checked() Layout {
	checked := Layout{Keys: l.Keys, Office: l.Office, Room: l.Room.checked(), Dial: l.Dial, Stick: l.Stick, Rows: []ButtonRow{}}
	seen := map[string]bool{}
	for _, row := range l.Rows[:min(len(l.Rows), maxRows)] {
		buttons := []Button{}
		for _, button := range row.Buttons {
			if b, ok := button.checked(); ok && !seen[b.ID] && len(buttons) < maxPerRow {
				seen[b.ID] = true
				buttons = append(buttons, b)
			}
		}
		if len(buttons) > 0 {
			checked.Rows = append(checked.Rows, ButtonRow{Tall: row.Tall, Buttons: buttons})
		}
	}
	return checked
}

func (b Button) checked() (Button, bool) {
	width := b.Width
	if width < 1 || width > maxWidth {
		width = 1
	}
	if slices.Contains(builtInButtons, b.ID) {
		return Button{ID: b.ID, Width: width}, true
	}
	if !madeButtonID.MatchString(b.ID) {
		return Button{}, false
	}
	// A command with a line break, or a key the deck can't press, is refused
	// whole: the editor can't make one, so it came from somewhere else.
	if _, known := buttonKeys[b.Key]; (b.Key != "" && !known) || controlCode.MatchString(b.Text) {
		return Button{}, false
	}
	checked := Button{ID: b.ID, Width: width, Label: oneLine(b.Label, labelLimit), Key: b.Key, WhenFree: b.WhenFree, Confirm: b.Confirm}
	if b.Key == "" {
		checked.Text = oneLine(b.Text, commandLimit)
	}
	for _, agent := range agents {
		if slices.Contains(b.Agents, string(agent)) {
			checked.Agents = append(checked.Agents, string(agent))
		}
	}
	if len(checked.Agents) == len(agents) {
		checked.Agents = nil
	}
	return checked, true
}

// oneLine is text without control characters or surrounding space, cut to
// limit characters.
func oneLine(text string, limit int) string {
	text = strings.TrimSpace(controlCode.ReplaceAllString(text, ""))
	if runes := []rune(text); len(runes) > limit {
		return string(runes[:limit])
	}
	return text
}

// madeButton is the button made in setup with id, if the layout has it.
func (l Layout) madeButton(id string) (Button, bool) {
	for _, row := range l.Rows {
		for _, b := range row.Buttons {
			if b.ID == id && !slices.Contains(builtInButtons, b.ID) {
				return b, true
			}
		}
	}
	return Button{}, false
}

// sameAs compares two layouts as the phone would see them.
func (l Layout) sameAs(other Layout) bool {
	a, _ := json.Marshal(l)
	b, _ := json.Marshal(other)
	return string(a) == string(b)
}

// Plan is the room arranged your way, from the setup page's editor: for each
// shape of the room, wide and tall, the pieces moved from where they stand,
// made bigger, or put away, and the pieces added to the lounge, as the Mac
// app's Plan. office.js draws it, with the same pieces and sizes. Anything
// unknown is left out, so the piece stands as it comes.
type Plan struct {
	Wide  map[string]Piece `json:"wide"`
	Tall  map[string]Piece `json:"tall"`
	Added Added            `json:"added"`
	Hung  Hung             `json:"hung"`
}

// Hung is what hangs on the back wall, for each shape of the room: each piece
// round its middle, where you put it. nil keeps the wall's five places, as set
// up; an empty list is a bare wall.
type Hung struct {
	Wide *[]HungPiece `json:"wide,omitempty"`
	Tall *[]HungPiece `json:"tall,omitempty"`
}

// HungPiece is one piece on the back wall, X being its middle.
type HungPiece struct {
	Kind string `json:"kind"`
	X    int    `json:"x"`
}

const (
	// hungMost is the most pieces the back wall holds, in each shape.
	hungMost = 12
	// wallLength is how far along the wall a piece can be: the back wall, and
	// held upright, the lounge's own wall after it, 224 on.
	wallLength = 448
	// wallMiddle is where a piece goes on the wall when it doesn't say.
	wallMiddle = 110
)

// Added is the pieces added to the lounge, for each shape of the room: any
// its corner can hold, the same one more than once, each moved from where the
// corner's piece stands, and grown.
type Added struct {
	Wide []AddedPiece `json:"wide"`
	Tall []AddedPiece `json:"tall"`
}

// AddedPiece is one piece added to the lounge.
type AddedPiece struct {
	Kind string  `json:"kind"`
	X    int     `json:"x"`
	Y    int     `json:"y"`
	Size float64 `json:"size"`
}

// addedMost is the most pieces that can be added to the lounge, in each shape.
const addedMost = 12

// Piece is how one piece is arranged: moved by X and Y, in the room's pixels,
// grown to Size, or put away.
type Piece struct {
	X    int     `json:"x"`
	Y    int     `json:"y"`
	Size float64 `json:"size"`
	Off  bool    `json:"off"`
	// A desk's own look, whoever sits at it: its workspace, monitor,
	// finish, chair and thing, unless their project has its own workspace
	// or thing, and the outfit and colour it gives whoever sits at it, over
	// their role's and the dress code's but not their own; "auto" for each
	// otherwise, and on what isn't a desk.
	Workspace string `json:"workspace"`
	Monitor   string `json:"monitor"`
	Finish    string `json:"finish"`
	Chair     string `json:"chair"`
	Thing     string `json:"thing"`
	Outfit    string `json:"outfit"`
	Colour    string `json:"colour"`
}

var (
	// What can be arranged: each desk, which moves and can be put away, and
	// the lounge's pieces, which can grow, and be put away but for the red
	// button and the mic, which are controls; and the door and the elevator,
	// which only move along the back wall.
	arrangeable = func() map[string]struct{ grows, hides bool } {
		pieces := map[string]struct{ grows, hides bool }{}
		for i := 1; i <= 9; i++ {
			pieces[fmt.Sprintf("desk%d", i)] = struct{ grows, hides bool }{false, true}
		}
		for _, part := range []string{"wall", "corner", "couch", "pong", "plant"} {
			pieces[part] = struct{ grows, hides bool }{true, true}
		}
		for _, part := range []string{"button", "mic"} {
			pieces[part] = struct{ grows, hides bool }{true, false}
		}
		// The lounge's window, which only moves along its wall
		pieces["window"] = struct{ grows, hides bool }{false, true}
		for _, part := range []string{"door", "elevator"} {
			pieces[part] = struct{ grows, hides bool }{false, false}
		}
		return pieces
	}()
	pieceSizes = []float64{1, 1.5, 2}
)

// pieceReach is how far a piece can be moved, either way, in the room's pixels.
const pieceReach = 400

// UnmarshalJSON keeps what it knows of the arrangement it's given, each piece,
// and each part of one, read on its own.
func (p *Plan) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)
	arranged := func(data json.RawMessage) map[string]Piece {
		var given map[string]json.RawMessage
		json.Unmarshal(data, &given)
		pieces := map[string]Piece{}
		for part, fields := range given {
			var parts map[string]json.RawMessage
			if _, ok := arrangeable[part]; !ok || json.Unmarshal(fields, &parts) != nil || parts == nil {
				continue
			}
			number := func(name string) float64 {
				var n float64
				json.Unmarshal(parts[name], &n)
				return n
			}
			var off bool
			json.Unmarshal(parts["off"], &off)
			text := func(name string) string {
				var given string
				json.Unmarshal(parts[name], &given)
				return given
			}
			pieces[part] = Piece{X: wholeWithin(number("x"), pieceReach), Y: wholeWithin(number("y"), pieceReach), Size: number("size"), Off: off,
				Workspace: text("workspace"), Monitor: text("monitor"), Finish: text("finish"), Chair: text("chair"), Thing: text("thing"),
				Outfit: text("outfit"), Colour: text("colour")}
		}
		return pieces
	}
	var added map[string]json.RawMessage
	json.Unmarshal(raw["added"], &added)
	pieces := func(data json.RawMessage) []AddedPiece {
		var given []json.RawMessage
		json.Unmarshal(data, &given)
		list := []AddedPiece{}
		for _, one := range given {
			var parts map[string]json.RawMessage
			if json.Unmarshal(one, &parts) != nil || parts == nil {
				continue
			}
			var kind string
			json.Unmarshal(parts["kind"], &kind)
			number := func(name string) float64 {
				var n float64
				json.Unmarshal(parts[name], &n)
				return n
			}
			list = append(list, AddedPiece{Kind: kind, X: wholeWithin(number("x"), pieceReach), Y: wholeWithin(number("y"), pieceReach), Size: number("size")})
		}
		return list
	}
	var hung map[string]json.RawMessage
	json.Unmarshal(raw["hung"], &hung)
	// A list for a wall as hung, or nil, for its five places, when there isn't one
	wall := func(data json.RawMessage) *[]HungPiece {
		var given []json.RawMessage
		if json.Unmarshal(data, &given) != nil || given == nil {
			return nil
		}
		list := []HungPiece{}
		for _, one := range given {
			var parts map[string]json.RawMessage
			if json.Unmarshal(one, &parts) != nil || parts == nil {
				continue
			}
			var kind string
			json.Unmarshal(parts["kind"], &kind)
			x := float64(wallMiddle)
			json.Unmarshal(parts["x"], &x)
			list = append(list, HungPiece{Kind: kind, X: wholeWithin(x, wallLength)})
		}
		return &list
	}
	*p = Plan{Wide: arranged(raw["wide"]), Tall: arranged(raw["tall"]), Added: Added{Wide: pieces(added["wide"]), Tall: pieces(added["tall"])},
		Hung: Hung{Wide: wall(hung["wide"]), Tall: wall(hung["tall"])}}
	return nil
}

// wholeWithin is n as a whole number no further than reach either side of 0:
// clamped before it's made whole, as converting a number beyond an int's range
// gives a different answer on each processor.
func wholeWithin(n float64, reach int) int {
	if math.IsNaN(n) {
		return 0
	}
	return int(math.Round(max(-float64(reach), min(float64(reach), n))))
}

// checked is the arrangement with every piece within reach, at a size it can
// be, and put away only if it can be.
func (p Plan) checked() Plan {
	pieces := func(given map[string]Piece) map[string]Piece {
		checked := map[string]Piece{}
		for part, piece := range given {
			kind, ok := arrangeable[part]
			if !ok {
				continue
			}
			piece.X = max(-pieceReach, min(pieceReach, piece.X))
			piece.Y = max(-pieceReach, min(pieceReach, piece.Y))
			if !kind.grows || !slices.Contains(pieceSizes, piece.Size) {
				piece.Size = 1
			}
			piece.Off = piece.Off && kind.hides
			desk := strings.HasPrefix(part, "desk")
			own := func(given string, choices []string) string {
				if desk && given != "auto" && slices.Contains(choices, given) {
					return given
				}
				return "auto"
			}
			piece.Workspace = own(piece.Workspace, workspaces)
			piece.Monitor = own(piece.Monitor, roomChoices["monitor"])
			piece.Finish = own(piece.Finish, roomChoices["desk"])
			piece.Chair = own(piece.Chair, roomChoices["chair"])
			piece.Thing = own(piece.Thing, teamChoices["desk"])
			piece.Outfit = own(piece.Outfit, teamChoices["outfit"])
			piece.Colour = own(piece.Colour, teamChoices["colour"])
			checked[part] = piece
		}
		return checked
	}
	added := func(given []AddedPiece) []AddedPiece {
		checked := []AddedPiece{}
		for _, piece := range given {
			if piece.Kind == "none" || !slices.Contains(roomChoices["corner"], piece.Kind) || len(checked) == addedMost {
				continue
			}
			piece.X = max(-pieceReach, min(pieceReach, piece.X))
			piece.Y = max(-pieceReach, min(pieceReach, piece.Y))
			if !slices.Contains(pieceSizes, piece.Size) {
				piece.Size = 1
			}
			checked = append(checked, piece)
		}
		return checked
	}
	hung := func(given *[]HungPiece) *[]HungPiece {
		if given == nil {
			return nil
		}
		checked := []HungPiece{}
		for _, piece := range *given {
			if piece.Kind == "bare" || !slices.Contains(wallChoices, piece.Kind) || len(checked) == hungMost {
				continue
			}
			piece.X = max(0, min(wallLength, piece.X))
			checked = append(checked, piece)
		}
		return &checked
	}
	return Plan{Wide: pieces(p.Wide), Tall: pieces(p.Tall), Added: Added{Wide: added(p.Added.Wide), Tall: added(p.Added.Tall)},
		Hung: Hung{Wide: hung(p.Hung.Wide), Tall: hung(p.Hung.Tall)}}
}
