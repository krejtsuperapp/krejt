import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';

import 'app.dart';
import 'state/app_state.dart';
import 'state/cart_state.dart';

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  SystemChrome.setSystemUIOverlayStyle(SystemUiOverlayStyle.light);
  runApp(
    MultiProvider(
      providers: [
        ChangeNotifierProvider<AppState>(create: (_) => AppState()..boot()),
        // Shporta jeton sa aplikacioni: dalja nga menuja nuk e humb atë që ke zgjedhur.
        ChangeNotifierProvider<CartState>(create: (_) => CartState()),
      ],
      child: const KrejtCustomerApp(),
    ),
  );
}
