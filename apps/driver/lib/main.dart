import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';

import 'app.dart';
import 'state/app_state.dart';
import 'state/work_state.dart';

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  SystemChrome.setSystemUIOverlayStyle(SystemUiOverlayStyle.light);
  final app = AppState()..boot();
  runApp(
    MultiProvider(
      providers: [
        ChangeNotifierProvider<AppState>.value(value: app),
        // Turni përdor të njëjtin klient API si pjesa tjetër e aplikacionit, që sesioni
        // dhe rifreskimi i token-it të mbeten në një vend të vetëm.
        ChangeNotifierProvider<WorkState>(create: (_) => WorkState(api: app.api)),
      ],
      child: const KrejtDriverApp(),
    ),
  );
}
