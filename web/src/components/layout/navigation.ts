import { MdSpaceDashboard, MdSettings, MdHistory , MdSync, MdArticle} from "react-icons/md";
import { IconType } from "react-icons";

interface NavItem {
  name: string;
  path: string;
  exact?: boolean;
  icon: IconType;
}

export const NAV_ROUTES: Array<NavItem> = [
  { name: "Dashboard", path: "/", exact: true, icon: MdSpaceDashboard },
  { name: "Sync Inspector", path: "/sync", icon: MdSync },
  { name: "Plex Payloads", path: "/plex-payloads", icon: MdHistory },
  { name: "Anime Updates", path: "/anime-updates", icon: MdSync },
  { name: "Logs", path: "/logs", icon: MdArticle },
  { name: "Settings", path: "/settings", icon: MdSettings },
];
