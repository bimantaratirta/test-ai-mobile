import 'package:flutter/widgets.dart';
import 'package:sentry_flutter/sentry_flutter.dart';

import '../../config/env.dart';

Future<void> initSentryAndRun(Widget app) async {
  final dsn = Env.sentryDsn;
  if (dsn.isEmpty) {
    runApp(app);
    return;
  }
  await SentryFlutter.init(
    (options) {
      options.dsn = dsn;
      options.tracesSampleRate = 0.1;
      options.environment = 'prototype';
    },
    appRunner: () => runApp(app),
  );
}
