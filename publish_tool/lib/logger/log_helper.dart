import 'dart:io';
import 'package:logging/logging.dart';
import 'package:path_provider/path_provider.dart';

class LogHelper {
  static final _logger = Logger('app');
  static IOSink? _fileSink;

  static Future<void> configureLogging() async {
    final appDir = await getApplicationSupportDirectory();
    final logsDir = Directory('${appDir.path}/logs');
    if (!logsDir.existsSync()) logsDir.createSync(recursive: true);

    final file = File('${logsDir.path}/log.log');
    _fileSink = file.openWrite(mode: FileMode.append);

    Logger.root.level = Level.ALL;
    Logger.root.onRecord.listen((record) {
      final line =
          '${record.time} [${record.level.name}] ${record.loggerName}: ${record.message}'
          '${record.error != null ? '\nError: ${record.error}' : ''}'
          '${record.stackTrace != null ? '\n${record.stackTrace}' : ''}';
      _fileSink?.writeln(line);
    });
  }

  static void debug(Object? msg) => _logger.fine(msg?.toString() ?? '');
  static void errorWithError(Object? err) => _logger.severe('', err);
  static void errorWithMsg(Object? msg) => _logger.severe(msg?.toString() ?? '');
  static void trace(Object? msg) => _logger.finest(msg?.toString() ?? '');
}
