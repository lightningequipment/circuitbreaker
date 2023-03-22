import Image from 'next/image';
import { InputBase } from '@mui/material';

interface SearchBarProps {
  placeholder: string;
  searchValue: string;
  setSearchValue: (value: string) => void;
}

const SearchBar = ({
  placeholder,
  searchValue,
  setSearchValue,
}: SearchBarProps) => (
  <InputBase
    startAdornment={
      <Image src="/icons/search-icon.svg" alt="search" width={13} height={13} />
    }
    placeholder={placeholder}
    fullWidth
    value={searchValue}
    onChange={(e) => {
      setSearchValue(e.target.value);
    }}
    sx={{
      height: '40px',
    }}
  />
);

export default SearchBar;
