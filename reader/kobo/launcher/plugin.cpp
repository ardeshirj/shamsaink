// ShamsaInk's rows in Nickel's More tab, for Kobo firmware 5.x/6.x (Qt 6), where NickelMenu does not run (ShamsaOS
// experiment 12, 2026-11-11). Built by build.sh into out/kobo/Kobo.tgz, which the firmware unpacks under
// /usr/local/Kobo, so the library lands in imageformats/ and Nickel loads it at its start.
// "ShamsaInk", above Help, shown while .adds/shamsaink/launch.sh is there: runs it, detached (it stops Nickel, runs the
// reader and starts Nickel again). Nothing else: the development SSH is switched from the reader's Settings, so the code
// in Nickel's process stays this small (2026-11-11).
//
// The way into Nickel follows NickelModManager (github.com/nicoverbruggen/NickelModManager, MIT License, Copyright (c)
// 2026 Nico Verbruggen): a Qt image plugin that decodes nothing, started by Q_COREAPP_STARTUP_FUNCTION in Nickel only;
// Nickel's widgets found by object name ("helpButton", "MainWindow", "moreButton"), not by translated labels; the row
// inserted above Help while Help is polished or shown, so it is in More's first paint; the row's painting adapted
// from its src/ui/menu.cpp.
//
// Start-up guard: at a start not yet confirmed for this library and this firmware, the library moves out of
// imageformats/ (Qt 6 loads any file there) to /usr/local/Kobo/libshamsaink-launcher.so.parked; once Home has stayed
// up 5 s it moves back and the start is recorded as confirmed. A crash before that leaves it parked, so the next start
// is a plain Kobo.
//
// Uninstalled: ShamsaInk's folder deleted over USB cannot take this library with it, so once Home is up and the
// storage is mounted (.kobo there) without .adds/shamsaink, the library deletes itself; the start after is a plain Kobo.
// Logged to .adds/shamsaink/logs/launcher.log.
#include <QApplication>
#include <QBoxLayout>
#include <QCoreApplication>
#include <QCryptographicHash>
#include <QDateTime>
#include <QDir>
#include <QEvent>
#include <QFile>
#include <QFileInfo>
#include <QImageIOPlugin>
#include <QLabel>
#include <QPainter>
#include <QPointer>
#include <QPolygonF>
#include <QPushButton>
#include <QTimer>
#include <cerrno>
#include <cstring>
#include <dlfcn.h>
#include <fcntl.h>
#include <sys/wait.h>
#include <unistd.h>

namespace {
const char kStorage[] = "/mnt/onboard/.kobo"; // there once the user storage is mounted
const char kDir[] = "/mnt/onboard/.adds/shamsaink";
const char kLog[] = "/mnt/onboard/.adds/shamsaink/logs/launcher.log";
const char kConfirmed[] = "/mnt/onboard/.adds/shamsaink/launcher.confirmed";
const char kLaunch[] = "/mnt/onboard/.adds/shamsaink/launch.sh";
const char kParked[] = "/usr/local/Kobo/libshamsaink-launcher.so.parked";

void note(const QByteArray &line) {
    if (access(kDir, F_OK) != 0) { // not installed (or the storage not mounted): nowhere to write
        return;
    }
    QDir().mkpath(QString::fromLatin1(kDir) + "/logs");
    QFile f(kLog);
    if (f.open(QIODevice::Append)) {
        f.write(QDateTime::currentDateTimeUtc().toString(Qt::ISODate).toUtf8() + " " + line + "\n");
        f.flush();
        fsync(f.handle());
    }
}

QString selfPath() {
    Dl_info info;
    if (!dladdr(reinterpret_cast<void *>(&selfPath), &info) || !info.dli_fname) {
        return {};
    }
    return QFileInfo(QFile::decodeName(info.dli_fname)).absoluteFilePath();
}

// What a confirmed start is recorded against: this library's bytes and the firmware build (revinfo).
QByteArray identity(const QString &library) {
    QCryptographicHash hash(QCryptographicHash::Sha256);
    QFile lib(library);
    if (lib.open(QIODevice::ReadOnly)) {
        hash.addData(&lib);
    }
    QFile rev("/usr/local/Kobo/revinfo");
    const QByteArray firmware = rev.open(QIODevice::ReadOnly) ? rev.readLine(256).trimmed() : QByteArray("?");
    return hash.result().toHex() + " " + firmware;
}

struct Guard {
    QString self;
    QByteArray id;
    bool armed = false;
};
Guard guard;

void confirmStart() {
    if (!guard.armed) {
        return;
    }
    const QByteArray self = QFile::encodeName(guard.self);
    if (access(self.constData(), F_OK) == 0) { // a new install arrived meanwhile: it is checked at its own start
        note("confirm: a library is already in place again; the parked one dropped");
        unlink(kParked);
    } else if (rename(kParked, self.constData()) != 0) {
        note(QByteArray("confirm: could not put the library back: ") + strerror(errno));
        return;
    }
    QFile rec(kConfirmed);
    if (rec.open(QIODevice::WriteOnly | QIODevice::Truncate)) {
        rec.write(guard.id + "\n");
        rec.flush();
        fsync(rec.handle());
    }
    sync();
    guard.armed = false;
    note("start confirmed: Home stayed up; the library is back in imageformats/");
}

// ShamsaInk uninstalled (its folder gone, the storage mounted): the library, in place or parked, deletes itself.
bool removeIfUninstalled() {
    if (access(kStorage, F_OK) != 0 || access(kDir, F_OK) == 0) {
        return false;
    }
    unlink(QFile::encodeName(guard.self).constData());
    unlink(kParked);
    sync();
    guard.armed = false;
    return true;
}

void launch() {
    if (access(kLaunch, R_OK) != 0) {
        note(QByteArray("tapped, but there is no ") + kLaunch);
        return;
    }
    note("tapped: starting launch.sh");
    const pid_t pid = fork();
    if (pid < 0) {
        note(QByteArray("fork failed: ") + strerror(errno));
        return;
    }
    if (pid == 0) { // async-signal-safe calls only until exec: Nickel has many threads
        setsid();
        if (fork() != 0) {
            _exit(0);
        }
        for (int fd = 3; fd < 1024; fd++) {
            close(fd);
        }
        const int null = open("/dev/null", O_RDWR);
        if (null >= 0) {
            dup2(null, 0);
            dup2(null, 1);
            dup2(null, 2);
        }
        execl("/bin/sh", "sh", kLaunch, static_cast<char *>(nullptr));
        _exit(127);
    }
    waitpid(pid, nullptr, 0);
}

// ShamsaInk's mark (docs/brand/mark.svg, a 64-unit square), drawn from its own coordinates: an eight-pointed star in
// ink, its inner outline and four dots in paper, an amber centre (a light grey on a black and white screen). Ink and
// paper swap while the row is held, as Nickel's icons do.
void drawMark(QPainter &painter, const QRect &column, const QColor &ink, const QColor &paper) {
    static const qreal outer[] = {32, 3, 40.49, 11.49, 52.51, 11.49, 52.51, 23.51, 61, 32, 52.51, 40.49, 52.51, 52.51,
                                  40.49, 52.51, 32, 61, 23.51, 52.51, 11.49, 52.51, 11.49, 40.49, 3, 32, 11.49, 23.51,
                                  11.49, 11.49, 23.51, 11.49};
    static const qreal inner[] = {32, 9, 38.74, 15.74, 48.26, 15.74, 48.26, 25.26, 55, 32, 48.26, 38.74, 48.26, 48.26,
                                  38.74, 48.26, 32, 55, 25.26, 48.26, 15.74, 48.26, 15.74, 38.74, 9, 32, 15.74, 25.26,
                                  15.74, 15.74, 25.26, 15.74};
    const qreal side = qMin(column.width(), column.height()) * 0.8;
    if (side < 8) {
        return;
    }
    painter.save();
    painter.setRenderHint(QPainter::Antialiasing);
    painter.translate(column.center().x() - side / 2, column.center().y() - side / 2);
    painter.scale(side / 64, side / 64);
    QPolygonF star, outline;
    for (int k = 0; k < 32; k += 2) {
        star << QPointF(outer[k], outer[k + 1]);
        outline << QPointF(inner[k], inner[k + 1]);
    }
    painter.setPen(Qt::NoPen);
    painter.setBrush(ink);
    painter.drawPolygon(star);
    painter.setPen(QPen(paper, 1.5, Qt::SolidLine, Qt::RoundCap, Qt::RoundJoin));
    painter.setBrush(Qt::NoBrush);
    painter.drawPolygon(outline);
    painter.setPen(Qt::NoPen);
    painter.setBrush(paper);
    for (const QPointF &dot : {QPointF(32, 22), QPointF(22, 32), QPointF(42, 32), QPointF(32, 42)}) {
        painter.drawEllipse(dot, 3.5, 3.5);
    }
    painter.setBrush(QColor(0xF2, 0xB5, 0x44));
    painter.drawEllipse(QPointF(32, 32), 4, 4);
    painter.restore();
}

// The row: Nickel's own More rows are drawn from Help's label and icon column (NickelModManager's MenuEntry, adapted).
class Row final : public QPushButton {
  public:
    Row(const QString &caption, QWidget *reference, QWidget *parent)
        : QPushButton(caption, parent), reference_(reference) {
        setFocusPolicy(Qt::NoFocus);
        setSizePolicy(QSizePolicy::Preferred, QSizePolicy::Fixed);
    }
    QSize sizeHint() const override {
        const int h = reference_ ? (reference_->height() > 1 ? reference_->height() : reference_->sizeHint().height())
                                 : fontMetrics().height() * 3;
        return QSize(QPushButton::sizeHint().width(), qMax(1, h));
    }

  protected:
    void paintEvent(QPaintEvent *) override {
        QPainter painter(this);
        auto *label = reference_ ? reference_->findChild<QLabel *>("label") : nullptr;
        auto *glyph = reference_ ? reference_->findChild<QLabel *>("pixmap") : nullptr;
        QColor ink = label ? label->palette().color(label->foregroundRole()) : palette().color(QPalette::WindowText);
        const QColor paper = reference_ ? reference_->palette().color(QPalette::Window) : palette().color(QPalette::Window);
        const QColor rule = ink;
        int left = qMax(16, height() / 4);
        if (label && label->isVisible()) {
            left = label->mapTo(reference_, label->contentsRect().topLeft()).x() + qMax(0, label->indent());
        }
        if (isDown()) { // Nickel's rows invert while held
            painter.fillRect(rect(), ink);
            ink = paper;
        }
        if (glyph && glyph->isVisible()) { // ShamsaInk's mark in the icon column
            const QRect column(glyph->mapTo(reference_, QPoint()), glyph->size());
            drawMark(painter, column, ink, paper);
        }
        painter.setFont(label ? label->font() : font());
        painter.setPen(ink);
        painter.drawText(QRect(left, 0, qMax(0, width() - left), height()), Qt::AlignLeft | Qt::AlignVCenter, text());
        painter.fillRect(QRect(0, height() - 1, width(), 1), rule);
    }

  private:
    QPointer<QWidget> reference_;
};

class Integration final : public QObject {
  public:
    explicit Integration(QObject *owner) : QObject(owner) {
        qApp->installEventFilter(this);
        queue();
    }

  private:
    bool queued_ = false;
    bool home_ = false;
    void inspect() {
        QWidget *main = nullptr;
        for (QWidget *w : QApplication::allWidgets()) {
            if (w->objectName() == "MainWindow" && QByteArray(w->metaObject()->className()) == "MainWindowView" &&
                w->isVisible()) {
                main = w;
            }
            insertRow(w);
        }
        if (main && !home_) {
            auto *more = main->findChild<QWidget *>("moreButton");
            if (more && more->isVisible()) {
                home_ = true;
                if (removeIfUninstalled()) { // nothing to log to: the folder is gone
                    return;
                }
                note("Home reached");
                QTimer::singleShot(5000, this, [] { confirmStart(); });
            }
        }
    }
    void insertRow(QWidget *help) {
        if (help->objectName() != "helpButton") {
            return;
        }
        auto *parent = help->parentWidget();
        auto *layout = parent ? qobject_cast<QBoxLayout *>(parent->layout()) : nullptr;
        if (!layout || layout->indexOf(help) < 0 || parent->findChild<QPushButton *>("shamsainkButton")) {
            return;
        }
        auto *row = new Row("ShamsaInk", help, parent);
        row->setObjectName("shamsainkButton");
        layout->insertWidget(layout->indexOf(help), row);
        connect(row, &QPushButton::clicked, this, [] { launch(); });
        note("row added to More");
        showRows(help);
    }
    // The row shown while launch.sh is there, each time More is shown.
    static void showRows(QWidget *help) {
        auto *parent = help->parentWidget();
        if (auto *row = parent ? parent->findChild<QPushButton *>("shamsainkButton") : nullptr) {
            row->setVisible(access(kLaunch, R_OK) == 0);
        }
    }
    void queue() {
        if (queued_) {
            return;
        }
        queued_ = true;
        QTimer::singleShot(0, this, [this] {
            queued_ = false;
            inspect();
        });
    }
    bool eventFilter(QObject *watched, QEvent *event) override {
        const auto type = event->type();
        if ((type == QEvent::Polish || type == QEvent::Show) && watched->isWidgetType() &&
            watched->objectName() == "helpButton") {
            insertRow(static_cast<QWidget *>(watched));
            if (type == QEvent::Show) {
                showRows(static_cast<QWidget *>(watched));
            }
        }
        if (type == QEvent::Show || type == QEvent::ChildAdded || type == QEvent::Polish) {
            queue();
        }
        return QObject::eventFilter(watched, event);
    }
};

void start() {
    if (!qobject_cast<QApplication *>(QCoreApplication::instance())) {
        return;
    }
    const QStringList args = QCoreApplication::arguments();
    if (args.isEmpty() || QFileInfo(args.first()).fileName() != "nickel") {
        return;
    }
    guard.self = selfPath();
    const QByteArray self = QFile::encodeName(guard.self);
    if (guard.self.isEmpty() || !dlopen(self.constData(), RTLD_LAZY | RTLD_NODELETE)) {
        note("cannot keep itself loaded: not started");
        return;
    }
    guard.id = identity(guard.self);
    QFile rec(kConfirmed);
    const QByteArray confirmed = rec.open(QIODevice::ReadOnly) ? rec.readLine(512).trimmed() : QByteArray();
    if (confirmed != guard.id) {
        unlink(kParked); // a library parked by an earlier failed start: this one replaces it
        if (rename(self.constData(), kParked) == 0) {
            sync();
            guard.armed = true;
            note("loaded in Nickel " + QByteArray::number(getpid()) + "; new library or firmware: guard armed (parked)");
        } else {
            note(QByteArray("could not park the library: ") + strerror(errno) + "; started without the guard");
        }
    } else {
        note("loaded in Nickel " + QByteArray::number(getpid()) + "; confirmed before");
    }
    new Integration(qApp);
}
} // namespace
Q_COREAPP_STARTUP_FUNCTION(start)

class ShamsaInkLauncherPlugin final : public QImageIOPlugin {
    Q_OBJECT
    Q_PLUGIN_METADATA(IID "org.qt-project.Qt.QImageIOHandlerFactoryInterface" FILE "plugin.json")
  public:
    Capabilities capabilities(QIODevice *, const QByteArray &) const override { return {}; }
    QImageIOHandler *create(QIODevice *, const QByteArray & = {}) const override { return nullptr; }
};
#include "plugin.moc"
