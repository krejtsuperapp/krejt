/// Klienti i API-së së KREJT (§39, §74). Pasqyron `backend/internal/platform/httpx/openapi/openapi.yaml`:
/// një zarf i vetëm gabimi, shuma vetëm si numra të plotë në cent, asnjë çmim apo status i dërguar nga klienti.
library;

export 'src/client.dart';
export 'src/errors.dart';
export 'src/session.dart';
export 'src/realtime.dart';
export 'src/money.dart';
export 'src/models/config.dart';
export 'src/models/user.dart';
export 'src/models/places.dart';
export 'src/models/ride.dart';
export 'src/models/wallet.dart';
export 'src/models/driver.dart';
export 'src/models/order.dart';
export 'src/models/parcel.dart';
